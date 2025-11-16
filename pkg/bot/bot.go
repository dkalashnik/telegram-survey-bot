package bot

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Client struct {
	api  *tgbotapi.BotAPI
	Self *tgbotapi.User
}

func NewClient(token string) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("bot token cannot be empty")
	}

	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot api instance: %w", err)
	}

	api.Debug = false

	log.Printf("Verifying API token...")
	ok, err := api.GetMe()
	if err != nil {
		return nil, fmt.Errorf("failed to verify bot token with GetMe(): %w", err)
	}
	log.Printf("Token verified successfully.")

	client := &Client{
		api:  api,
		Self: &ok,
	}

	return client, nil
}

func (c *Client) SendMessage(chatID int64, text string, markup interface{}) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, text)

	msg.ParseMode = ""

	if markup != nil {
		msg.ReplyMarkup = markup
	}

	sentMsg, err := c.api.Send(msg)
	if err != nil {
		return tgbotapi.Message{}, fmt.Errorf("failed to send message: %w", err)
	}
	return sentMsg, nil
}

func (c *Client) EditMessageText(chatID int64, messageID int, text string, markup *tgbotapi.InlineKeyboardMarkup) (tgbotapi.Message, error) {
	if messageID == 0 {
		log.Printf("Warning: EditMessageText called with messageID=0 for chat %d. Sending new message instead.", chatID)
		return c.SendMessage(chatID, text, markup)
	}

	msg := tgbotapi.NewEditMessageText(chatID, messageID, text)

	msg.ParseMode = ""
	if markup != nil {
		msg.ReplyMarkup = markup
	}

	sentMsg, err := c.api.Send(msg)
	if err != nil {

		if err.Error() == "Bad Request: message is not modified: specified new message content and reply markup are exactly the same as a current content and reply markup of the message" {
			log.Printf("Message %d in chat %d was not modified, ignoring error.", messageID, chatID)

			return tgbotapi.Message{MessageID: messageID, Chat: &tgbotapi.Chat{ID: chatID}}, nil
		}
		return tgbotapi.Message{}, fmt.Errorf("failed to edit message %d: %w", messageID, err)
	}
	return sentMsg, nil
}

func (c *Client) AnswerCallback(callbackID string, text string) error {
	if callbackID == "" {
		return fmt.Errorf("callbackID cannot be empty")
	}
	callbackCfg := tgbotapi.NewCallback(callbackID, text)

	_, err := c.api.Request(callbackCfg)
	if err != nil {
		return fmt.Errorf("failed to answer callback query %s: %w", callbackID, err)
	}
	return nil
}

func (c *Client) RemoveReplyKeyboard(chatID int64, text string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, text)

	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)

	sentMsg, err := c.api.Send(msg)
	if err != nil {
		return tgbotapi.Message{}, fmt.Errorf("failed to send remove keyboard message: %w", err)
	}
	return sentMsg, nil
}

func (c *Client) GetUpdatesChan(timeout int) tgbotapi.UpdatesChannel {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = timeout

	return c.api.GetUpdatesChan(u)
}

func (c *Client) SendTypingAction(chatID int64) error {
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	_, err := c.api.Request(action)
	if err != nil {
		return fmt.Errorf("failed to send typing action: %w", err)
	}
	return nil
}

func (c *Client) PinMessage(chatID int64, messageID int, disableNotification bool) error {
	pinConfig := tgbotapi.PinChatMessageConfig{
		ChatID:              chatID,
		MessageID:           messageID,
		DisableNotification: disableNotification,
	}
	_, err := c.api.Request(pinConfig)
	if err != nil {
		return fmt.Errorf("failed to pin message %d: %w", messageID, err)
	}
	return nil
}

func (c *Client) UnpinMessage(chatID int64, messageID int) error {

	unpinConfig := tgbotapi.UnpinChatMessageConfig{
		ChatID:    chatID,
		MessageID: messageID,
	}
	_, err := c.api.Request(unpinConfig)
	if err != nil {
		return fmt.Errorf("failed to unpin message %d: %w", messageID, err)
	}
	return nil
}
