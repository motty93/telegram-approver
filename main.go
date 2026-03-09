package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"regexp"
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

type Chat struct {
	ID int64 `json:"id"`
}

type Message struct {
	MessageID      int64    `json:"message_id"`
	Chat           Chat     `json:"chat"`
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

// HookInput represents the JSON input from Claude Code PreToolUse hook.
type HookInput struct {
	ToolName string          `json:"tool_name"`
	Input    json.RawMessage `json:"tool_input"`
}

type BashInput struct {
	Command string `json:"command"`
}

type FileInput struct {
	FilePath string `json:"file_path"`
}

var dangerousPattern = regexp.MustCompile(`(rm |sudo|deploy|terraform|docker|kubectl|gcloud|aws|git push|dd |mkfs|dropdb)`)

const memoryPathPattern = "/.claude/projects/"

func hookAllow(reason string) {
	fmt.Printf(`{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","permissionDecisionReason":"%s"}}`, reason)
	os.Exit(0)
}

func hookDeny() {
	os.Exit(2)
}

func runHook() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Println("failed to read stdin:", err)
		hookAllow("Auto-approved (stdin error)")
		return
	}

	var hook HookInput
	if err := json.Unmarshal(input, &hook); err != nil {
		log.Println("failed to parse hook input:", err)
		hookAllow("Auto-approved (parse error)")
		return
	}

	log.Println("TOOL:", hook.ToolName)

	switch hook.ToolName {
	case "Bash":
		var bi BashInput
		if err := json.Unmarshal(hook.Input, &bi); err != nil {
			hookAllow("Auto-approved (parse error)")
			return
		}
		log.Println("COMMAND:", bi.Command)
		if dangerousPattern.MatchString(bi.Command) {
			requestHookApproval(bi.Command)
		} else {
			hookAllow("Auto-approved by hook")
		}

	case "Edit", "Write":
		var fi FileInput
		if err := json.Unmarshal(hook.Input, &fi); err != nil {
			hookAllow("Auto-approved (parse error)")
			return
		}
		log.Println("FILE:", fi.FilePath)
		if strings.Contains(fi.FilePath, memoryPathPattern) {
			hookAllow("Auto-approved by hook")
		} else {
			requestHookApproval(fmt.Sprintf("[%s] %s", hook.ToolName, fi.FilePath))
		}

	default:
		hookAllow("Auto-approved by hook")
	}
}

func requestHookApproval(message string) {
	log.Println("requesting approval:", message)

	token := os.Getenv("TELEGRAM_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chatID == "" {
		log.Println("TELEGRAM_TOKEN / TELEGRAM_CHAT_ID not set, auto-approving")
		hookAllow("Auto-approved (no credentials)")
		return
	}

	exitCode := runApproval(token, chatID, message)
	if exitCode == 0 {
		hookAllow("Approved via Telegram")
	} else {
		hookDeny()
	}
}

// runApproval sends a message and waits for approval. Returns 0 for approved, 1 for denied/error.
func runApproval(token, chatID, message string) int {
	offset := getLatestOffset(token)

	sent, err := sendMessage(token, chatID, message)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sendMessage error:", err)
		return 1
	}

	fmt.Fprintln(os.Stderr, "message sent:", sent.MessageID)
	fmt.Fprintln(os.Stderr, "waiting for approval...")

	deadline := time.After(defaultTimeout)
	retryCount := 0

	for {
		select {
		case <-deadline:
			fmt.Fprintln(os.Stderr, "timeout: no approval received within", defaultTimeout)
			return 1
		default:
		}

		updates, err := getUpdates(token, offset)
		if err != nil {
			retryCount++
			if retryCount > maxRetries {
				fmt.Fprintln(os.Stderr, "getUpdates error (retries exhausted):", err)
				return 1
			}
			fmt.Fprintf(os.Stderr, "getUpdates error (retry %d/%d): %v\n", retryCount, maxRetries, err)
			time.Sleep(retryBaseInterval * (1 << time.Duration(retryCount)))
			continue
		}
		retryCount = 0

		for _, u := range updates {
			offset = u.UpdateID + 1
			if u.Message == nil {
				continue
			}

			if fmt.Sprintf("%d", u.Message.Chat.ID) != chatID {
				continue
			}

			isReply := u.Message.ReplyToMessage != nil &&
				u.Message.ReplyToMessage.MessageID == sent.MessageID
			isDirectMessage := u.Message.ReplyToMessage == nil &&
				u.Message.MessageID > sent.MessageID

			if !isReply && !isDirectMessage {
				continue
			}

			text := strings.ToUpper(strings.TrimSpace(u.Message.Text))
			switch text {
			case "OK":
				fmt.Fprintln(os.Stderr, "approved")
				return 0
			case "いいえ":
				fmt.Fprintln(os.Stderr, "denied")
				return 1
			}
		}
	}
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(0)

	if len(os.Args) > 1 && os.Args[1] == "hook" {
		runHook()
		return
	}

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

	os.Exit(runApproval(token, chatID, message))
}
