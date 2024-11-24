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

// Flan-T5 response structure
type FlanT5Response []struct {
	GeneratedText string `json:"generated_text"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	apiToken := os.Getenv("HF_API_TOKEN")

	if telegramToken == "" || apiToken == "" {
		log.Fatal("Telegram API token or Hugging Face API token not set")
	}

	// Initialize Telegram bot
	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Fatalf("Error creating bot: %v", err)
	}

	bot.Debug = true
	log.Printf("Authorized as %s", bot.Self.UserName)

	// Delete existing webhook if present
	err = deleteWebhook(telegramToken)
	if err != nil {
		log.Printf("Error deleting webhook: %v", err)
	}

	// Set new webhook URL for Vercel
	webhookURL := "https://your-vercel-url.com" // Replace with your actual Vercel webhook URL
	err = setWebhook(bot, webhookURL)
	if err != nil {
		log.Printf("Error setting webhook: %v", err)
	}

	// Handle incoming updates from Telegram
	update := tgbotapi.Update{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&update); err != nil {
		log.Printf("Error decoding update: %v", err)
		http.Error(w, "Error decoding update", http.StatusBadRequest)
		return
	}

	// Handle the message
	handleMessage(bot, update, apiToken)
}

func deleteWebhook(telegramToken string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/deleteWebhook", telegramToken)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to send deleteWebhook request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete webhook, received status code: %d", resp.StatusCode)
	}

	log.Println("Webhook deleted successfully")
	return nil
}

func setWebhook(bot *tgbotapi.BotAPI, webhookURL string) error {
	_, err := bot.SetWebhook(tgbotapi.NewWebhook(webhookURL))
	if err != nil {
		return fmt.Errorf("failed to set webhook: %v", err)
	}
	log.Printf("Webhook set successfully to %s", webhookURL)
	return nil
}

func handleMessage(bot *tgbotapi.BotAPI, update tgbotapi.Update, apiToken string) {
	// Ensure the update contains a message
	if update.Message == nil {
		log.Printf("Update does not contain a message. Ignoring.")
		return
	}

	// Log the received message
	log.Printf("[%s] Received message: %s", update.Message.From.UserName, update.Message.Text)

	// Handle /start command
	if strings.HasPrefix(update.Message.Text, "/start") {
		// Respond with a welcome message
		welcomeMessage := "Why do you bother me? ._."
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, welcomeMessage)
		if _, err := bot.Send(msg); err != nil {
			log.Printf("Error sending welcome message: %v", err)
		} else {
			log.Printf("Successfully sent welcome message to user")
		}
		return
	}

	// Send "typing" action
	action := tgbotapi.NewChatAction(update.Message.Chat.ID, tgbotapi.ChatTyping)
	if _, err := bot.Send(action); err != nil {
		log.Printf("Error sending typing action: %v", err)
	}

	// Get response from Flan-T5
	log.Printf("Sending request to Flan-T5 API...")
	response, err := getFlanT5Response(apiToken, update.Message.Text)
	if err != nil {
		log.Printf("Error getting Flan-T5 response: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I'm having trouble processing your message. Please try again later.")
		bot.Send(msg)
		return
	}

	log.Printf("Received response from Flan-T5: %s", response)

	// Send response back to user
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, response)
	msg.ReplyToMessageID = update.Message.MessageID

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	} else {
		log.Printf("Successfully sent response to user")
	}
}

func getFlanT5Response(apiToken, inputText string) (string, error) {
	const maxRetries = 3
	const retryDelay = 5 * time.Second
	const apiURL = "https://api-inference.huggingface.co/models/google/flan-t5-large"

	roles := []string{"What is the biggest lie in the universe?", "Pretend you are a witty person.", "Why did the spider use the computer?"}

	payload := map[string]interface{}{
		"inputs": roles[rand.Intn(len(roles))] + inputText,
		"parameters": map[string]interface{}{
			"max_length":  150,
			"temperature": 0.7,
			"top_p":       0.85,
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error marshalling payload: %v", err)
	}

	log.Printf("Request payload: %s", string(jsonPayload))

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("Retry attempt %d of %d", attempt+1, maxRetries)
			time.Sleep(retryDelay)
		}

		log.Printf("Sending request to HuggingFace API (attempt %d)...", attempt+1)
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("HTTP request error: %v", err)
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("error reading response: %v", err)
		}

		log.Printf("Response status: %d", resp.StatusCode)
		log.Printf("Response body: %s", string(body))

		if resp.StatusCode == 200 {
			// Parse the response
			var flanResponse FlanT5Response
			if err := json.Unmarshal(body, &flanResponse); err != nil || len(flanResponse) == 0 {
				return "", fmt.Errorf("failed to parse response: %s", string(body))
			}
			return flanResponse[0].GeneratedText, nil
		}

		log.Printf("Unexpected response status: %d", resp.StatusCode)
	}

	return "", fmt.Errorf("failed to get response after %d attempts", maxRetries)
}

func main() {
	http.HandleFunc("/", Handler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
