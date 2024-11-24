package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

var jokes = []string{
	"What was the spider doing on the computer? Making a web-site!",
	"How does a computer get drunk? It takes screen shots.",
	"What shoes do computers love the most? Re-boots!",
	"Autocorrect can go straight to he’ll.",
	"What is the biggest lie in the universe? I have read and agreed to the terms and conditions.",
}

type AIResponse []struct {
	GeneratedText string `json:"generated_text"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received a request")

	// Initialize Telegram bot
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Parse incoming request
	var update tgbotapi.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Printf("Failed to decode update: %v", err)
		return
	}

	if update.Message == nil { // Ignore non-message updates
		return
	}

	// Handle /start command
	if update.Message.Text == "/start" {
		sendWelcomeMessage(bot, update.Message.Chat.ID)
		return
	}

	// Handle AI query
	response, err := getAIResponse(update.Message.Text)
	if err != nil {
		log.Printf("Error getting AI response: %v", err)
		sendErrorMessage(bot, update.Message.Chat.ID)
		return
	}

	// Send the AI-generated response
	sendMessage(bot, update.Message.Chat.ID, response)

	fmt.Fprintf(w, "Update processed")
}

func sendWelcomeMessage(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Welcome! Type anything to receive an AI-generated response.")
	bot.Send(msg)
}

func sendErrorMessage(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Sorry, I couldn't process your request. Please try again later.")
	bot.Send(msg)
}

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	bot.Send(msg)
}

func getAIResponse(inputText string) (string, error) {
	const apiURL = "https://api-inference.huggingface.co/models/google/flan-t5-large"

	payload := map[string]interface{}{
		"inputs": inputText,
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

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+os.Getenv("HF_API_TOKEN"))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("non-200 status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var aiResponse AIResponse
	if err := json.NewDecoder(resp.Body).Decode(&aiResponse); err != nil {
		return "", fmt.Errorf("error decoding response: %v", err)
	}

	if len(aiResponse) == 0 || aiResponse[0].GeneratedText == "" {
		return "", fmt.Errorf("no text in AI response")
	}

	return aiResponse[0].GeneratedText, nil
}
