package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultTimeout    = 10 * time.Minute
	maxRetries        = 5
	retryBaseInterval = 2 * time.Second
)

const (
	telegramUpdateAPIURL = "https://api.telegram.org/bot%s/getUpdates"
	telegramSendAPIURL   = "https://api.telegram.org/bot%s/sendMessage"
	queryParameter       = "?timeout=30&offset=%d"
)

var httpClient = &http.Client{
	Timeout: 40 * time.Second,
}

type UpdateResponse struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	MessageID      int64    `json:"message_id"`
	Text           string   `json:"text"`
	ReplyToMessage *Message `json:"reply_to_message"`
}

type SendResponse struct {
	OK     bool    `json:"ok"`
	Result Message `json:"result"`
}

// sendMessage sends a Telegram message and returns the created message object.
func sendMessage(token, chatID, text string) (Message, error) {
	form := neturl.Values{}
	form.Add("chat_id", chatID)
	form.Add("text", text)

	endpoint := fmt.Sprintf(telegramSendAPIURL, token)

	resp, err := httpClient.PostForm(endpoint, form)
	if err != nil {
		return Message{}, fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Message{}, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return Message{}, fmt.Errorf("sendMessage failed (status %d): %s", resp.StatusCode, string(body))
	}

	var r SendResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return Message{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if !r.OK {
		return Message{}, fmt.Errorf("telegram API returned error: %s", string(body))
	}

	return r.Result, nil
}

// getUpdates retrieves Telegram updates using long polling.
func getUpdates(token string, offset int64) ([]Update, error) {
	endpoint := fmt.Sprintf(
		telegramUpdateAPIURL+queryParameter,
		token,
		offset,
	)

	resp, err := httpClient.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getUpdates status=%d body=%s", resp.StatusCode, string(body))
	}

	var r UpdateResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}

	if !r.OK {
		return nil, fmt.Errorf("telegram API error: %s", string(body))
	}

	return r.Result, nil
}

// getLatestOffset reads the latest update_id so old messages are ignored.
func getLatestOffset(token string) int64 {
	endpoint := fmt.Sprintf(telegramUpdateAPIURL, token)

	resp, err := httpClient.Get(endpoint)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0
	}

	if resp.StatusCode != http.StatusOK {
		return 0
	}

	var r UpdateResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return 0
	}

	var offset int64
	for _, u := range r.Result {
		if u.UpdateID >= offset {
			offset = u.UpdateID + 1
		}
	}

	return offset
}

func main() {
	token := os.Getenv("TELEGRAM_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if token == "" || chatID == "" {
		fmt.Fprintln(os.Stderr, "TELEGRAM_TOKEN / TELEGRAM_CHAT_ID required")
		os.Exit(1)
	}

	message := "Claude approval OK / いいえ"
	if len(os.Args) > 1 {
		message = strings.Join(os.Args[1:], " ")
	}

	offset := getLatestOffset(token)

	sent, err := sendMessage(token, chatID, message)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sendMessage error:", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "message sent:", sent.MessageID)
	fmt.Fprintln(os.Stderr, "waiting for approval...")

	deadline := time.After(defaultTimeout)
	retryCount := 0

	for {
		select {
		case <-deadline:
			fmt.Fprintln(os.Stderr, "timeout: no approval received within", defaultTimeout)
			os.Exit(1)
		default:
		}

		updates, err := getUpdates(token, offset)
		if err != nil {
			retryCount++
			if retryCount > maxRetries {
				fmt.Fprintln(os.Stderr, "getUpdates error (retries exhausted):", err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "getUpdates error (retry %d/%d): %v\n", retryCount, maxRetries, err)
			time.Sleep(retryBaseInterval * time.Duration(retryCount))
			continue
		}
		retryCount = 0

		for _, u := range updates {
			offset = u.UpdateID + 1
			if u.Message == nil || u.Message.ReplyToMessage == nil {
				continue
			}
			if u.Message.ReplyToMessage.MessageID != sent.MessageID {
				continue
			}

			text := strings.ToUpper(strings.TrimSpace(u.Message.Text))
			switch text {
			case "OK":
				fmt.Fprintln(os.Stderr, "approved")
				os.Exit(0)
			case "いいえ":
				fmt.Fprintln(os.Stderr, "denied")
				os.Exit(1)
			}
		}
	}
}
