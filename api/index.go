package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// FlanT5Response represents the structure of the Hugging Face API response
type FlanT5Response []struct {
	GeneratedText string `json:"generated_text"`
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func main() {
	telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	apiToken := os.Getenv("HF_API_TOKEN")
	webhookURL := os.Getenv("WEBHOOK_URL") // Set this in your environment variables

	if telegramToken == "" || apiToken == "" || webhookURL == "" {
		log.Fatal("Missing required environment variables (TELEGRAM_BOT_TOKEN, HF_API_TOKEN, WEBHOOK_URL)")
	}

	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Fatalf("Error creating Telegram bot: %v", err)
	}

	bot.Debug = true
	log.Printf("Authorized as %s", bot.Self.UserName)

	if err := setupWebhook(bot, webhookURL); err != nil {
		log.Fatalf("Error setting up webhook: %v", err)
	}

	// Start HTTP server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleRequest(w, r, bot, apiToken)
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// setupWebhook configures the webhook for Telegram bot
func setupWebhook(bot *tgbotapi.BotAPI, webhookURL string) error {
	// Delete existing webhook
	_, err := bot.RemoveWebhook()
	if err != nil {
		return fmt.Errorf("failed to remove webhook: %v", err)
	}

	// Set new webhook
	_, err = bot.SetWebhook(tgbotapi.NewWebhook(webhookURL))
	if err != nil {
		return fmt.Errorf("failed to set webhook: %v", err)
	}

	log.Printf("Webhook set to %s", webhookURL)
	return nil
}

// handleRequest processes incoming HTTP requests from Telegram
func handleRequest(w http.ResponseWriter, r *http.Request, bot *tgbotapi.BotAPI, apiToken string) {
	var update tgbotapi.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Failed to parse update", http.StatusBadRequest)
		log.Printf("Error decoding update: %v", err)
		return
	}

	if update.Message == nil {
		log.Printf("No message in update, ignoring.")
		return
	}

	log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

	// Handle commands and messages
	if strings.HasPrefix(update.Message.Text, "/start") {
		sendMessage(bot, update.Message.Chat.ID, "Why do you bother me? ._.")
	} else {
		processUserMessage(bot, update.Message, apiToken)
	}
}

// sendMessage sends a plain text message to a chat
func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

// processUserMessage handles user messages and gets a response from Flan-T5
func processUserMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message, apiToken string) {
	sendTypingAction(bot, message.Chat.ID)

	response, err := getFlanT5Response(apiToken, message.Text)
	if err != nil {
		log.Printf("Error getting response from Flan-T5: %v", err)
		sendMessage(bot, message.Chat.ID, "I'm having trouble processing your message. Please try again later.")
		return
	}

	sendMessage(bot, message.Chat.ID, response)
}

// sendTypingAction sends a "typing" action to the chat
func sendTypingAction(bot *tgbotapi.BotAPI, chatID int64) {
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	if _, err := bot.Send(action); err != nil {
		log.Printf("Error sending typing action: %v", err)
	}
}

// getFlanT5Response sends a request to the Hugging Face API and retrieves the response
func getFlanT5Response(apiToken, inputText string) (string, error) {
	const apiURL = "https://api-inference.huggingface.co/models/google/flan-t5-large"

	roles := []string{
		"What is the biggest lie in the universe? ",
		"Pretend you are a witty person. ",
		"Why did the spider use the computer? ",
	}
	payload := map[string]interface{}{
		"inputs": roles[rand.Intn(len(roles))] + inputText,
		"parameters": map[string]interface{}{
			"max_length":  150,
			"temperature": 0.7,
			"top_p":       0.85,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error marshaling payload: %v", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected response status: %d, body: %s", resp.StatusCode, string(body))
	}

	var flanResponse FlanT5Response
	if err := json.Unmarshal(body, &flanResponse); err != nil || len(flanResponse) == 0 {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	return flanResponse[0].GeneratedText, nil
}
