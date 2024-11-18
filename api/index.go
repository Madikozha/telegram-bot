package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

var jokes = []string{
	"I am broken temporalitawd ._.",
}

func Handler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received a request")

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	var update tgbotapi.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Printf("Failed to decode update: %v", err)
		return
	}

	if update.Message == nil { // ignore non-message updates
		return
	}

	// Check for the command or text to trigger the keyboard
	// if update.Message.Text == "/start" {
	// 	sendKeyboard(bot, update.Message.Chat.ID)
	// 	return
	// }

	// Check for the specific message to respond with a joke
	sendRandomJoke(bot, update.Message.Chat.ID)
	// Default response
	// msg := tgbotapi.NewMessage(update.Message.Chat.ID, "I didn't understand that. Try saying 'yes'.\nExample: Yes or yes and etc.")
	// bot.Send(msg)

	fmt.Fprintf(w, "Update processed")
}

func sendKeyboard(bot *tgbotapi.BotAPI, chatID int64) {
	buttons := []tgbotapi.KeyboardButton{
		tgbotapi.NewKeyboardButton("Yes"),
	}

	keyboard := tgbotapi.NewReplyKeyboard(buttons)
	keyboard.ResizeKeyboard = true

	msg := tgbotapi.NewMessage(chatID, "Do you want some jokes? UwU")
	msg.ReplyMarkup = keyboard

	bot.Send(msg)
}

func sendRandomJoke(bot *tgbotapi.BotAPI, chatID int64) {
	randomIndex := rand.Intn(len(jokes)) // Generate a random index
	joke := jokes[randomIndex]           // Select a random joke

	msg := tgbotapi.NewMessage(chatID, joke)
	bot.Send(msg)
}

func main() {
	http.HandleFunc("/", Handler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
