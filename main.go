package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var (
	queryParameter = "?timeout=30&offset=%d"
)

const (
	TELEGRAM_UPDATE_API_URL = "https://api.telegram.org/bot%s/getUpdates"
	TELEGRAM_SEND_API_URL   = "https://api.telegram.org/bot%s/sendMessage"
)

type UpdateResponse struct {
	Result []Update `json:"result"`
}

type Update struct {
	UpdateID int64   `json:"update_id"`
	Message  Message `json:"message"`
}

type Message struct {
	MessageID      int64    `json:"message_id"`
	Text           string   `json:"text"`
	ReplyToMessage *Message `json:"reply_to_message"`
}

type SendResponse struct {
	Result Message `json:"result"`
}

func sendMessage(token, chatID, text string) (Message, error) {
	form := url.Values{}
	form.Add("chat_id", chatID)
	form.Add("text", text)

	url := fmt.Sprintf(TELEGRAM_SEND_API_URL, token)
	resp, err := http.PostForm(
		url,
		form,
	)
	if err != nil {
		return Message{}, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Message{}, err
	}

	var r SendResponse
	json.Unmarshal(body, &r)

	return r.Result, nil
}

func getUpdates(token string, offset int64) ([]Update, error) {
	url := fmt.Sprintf(
		TELEGRAM_UPDATE_API_URL+queryParameter,
		token,
		offset,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var r UpdateResponse
	json.Unmarshal(body, &r)

	return r.Result, nil
}

func getLatestOffset(token string) int64 {
	url := fmt.Sprintf(
		TELEGRAM_UPDATE_API_URL,
		token,
	)

	resp, err := http.Get(url)
	if err != nil {
		return 0
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0
	}

	var r UpdateResponse
	json.Unmarshal(body, &r)

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
		fmt.Println("TELEGRAM_TOKEN / TELEGRAM_CHAT_ID required")
		os.Exit(1)
	}

	message := "Claude approval required\n返信: OK / いいえ"
	if len(os.Args) > 1 {
		message = strings.Join(os.Args[1:], " ")
	}

	offset := getLatestOffset(token)
	sent, err := sendMessage(token, chatID, message)
	if err != nil {
		panic(err)
	}

	fmt.Println("message sent:", sent.MessageID)
	fmt.Println("waiting for approval...")

	for {
		updates, err := getUpdates(token, offset)
		if err != nil {
			panic(err)
		}

		for _, u := range updates {
			offset = u.UpdateID + 1
			msg := u.Message
			if msg.ReplyToMessage == nil {
				continue
			}

			if msg.ReplyToMessage.MessageID != sent.MessageID {
				continue
			}

			text := strings.TrimSpace(msg.Text)
			switch text {
			case "OK":
				fmt.Println("approved")
				os.Exit(0)
			case "いいえ":
				fmt.Println("denied")
				os.Exit(1)
			}
		}
	}
}
