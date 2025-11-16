package telegramadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/dkalashnik/telegram-survey-bot/pkg/bot"
	"github.com/dkalashnik/telegram-survey-bot/pkg/ports/botport"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Package telegramadapter implements botport.BotPort using the existing Telegram client.
// See PRPs/ai_docs/botport_hex_adapter.md for naming conventions and error semantics.

// Logger defines the minimal logging interface used by the adapter.
type Logger interface {
	Printf(format string, args ...any)
}

type telegramClient interface {
	SendMessage(chatID int64, text string, markup interface{}) (tgbotapi.Message, error)
	EditMessageText(chatID int64, messageID int, text string, markup *tgbotapi.InlineKeyboardMarkup) (tgbotapi.Message, error)
	AnswerCallback(callbackID string, text string) error
}

// Adapter wraps a Telegram client and satisfies botport.BotPort.
type Adapter struct {
	client telegramClient
	logger Logger
}

var _ telegramClient = (*bot.Client)(nil)
var _ botport.BotPort = (*Adapter)(nil)

// New constructs a Telegram adapter with the provided bot client and logger.
func New(client telegramClient, logger Logger) (*Adapter, error) {
	if client == nil {
		return nil, fmt.Errorf("telegramadapter: client is nil")
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Adapter{
		client: client,
		logger: logger,
	}, nil
}

// SendMessage dispatches a new Telegram message and returns a botport.BotMessage record.
func (a *Adapter) SendMessage(ctx context.Context, chatID int64, text string, markup interface{}) (botport.BotMessage, error) {
	if err := ctx.Err(); err != nil {
		return botport.BotMessage{}, wrapContextError("send_message", err)
	}
	msg, err := a.client.SendMessage(chatID, text, markup)
	if err != nil {
		return botport.BotMessage{}, a.wrapAndLogError("send_message", chatID, 0, err)
	}
	bm := toBotMessage(msg, markup)
	a.log("send_message", map[string]any{"chat_id": bm.ChatID, "message_id": bm.MessageID})
	return bm, nil
}

// EditMessage edits an existing Telegram message.
func (a *Adapter) EditMessage(ctx context.Context, chatID int64, messageID int, text string, markup interface{}) (botport.BotMessage, error) {
	if err := ctx.Err(); err != nil {
		return botport.BotMessage{}, wrapContextError("edit_message", err)
	}
	inlineMarkup, err := toInlineKeyboard(markup)
	if err != nil {
		return botport.BotMessage{}, botport.NewBotError("edit_message", "bad_payload", err)
	}
	msg, err := a.client.EditMessageText(chatID, messageID, text, inlineMarkup)
	if err != nil {
		return botport.BotMessage{}, a.wrapAndLogError("edit_message", chatID, messageID, err)
	}
	bm := toBotMessage(msg, inlineMarkup)
	a.log("edit_message", map[string]any{"chat_id": bm.ChatID, "message_id": bm.MessageID})
	return bm, nil
}

// AnswerCallback acknowledges a callback query without contacting Telegram API directly in strategies.
func (a *Adapter) AnswerCallback(ctx context.Context, callbackID string, text string) error {
	if err := ctx.Err(); err != nil {
		return wrapContextError("answer_callback", err)
	}
	if err := a.client.AnswerCallback(callbackID, text); err != nil {
		return a.wrapAndLogError("answer_callback", 0, 0, err)
	}
	a.log("answer_callback", map[string]any{"callback_id": callbackID})
	return nil
}

func (a *Adapter) wrapAndLogError(op string, chatID int64, messageID int, err error) error {
	wrapped := wrapTelegramError(op, err)
	a.log(op, map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"code":       getBotErrorCode(wrapped),
		"error":      err.Error(),
	})
	return wrapped
}

func (a *Adapter) log(op string, attrs map[string]any) {
	if a.logger == nil {
		return
	}
	a.logger.Printf("botport op=%s attrs=%v", op, attrs)
}

func toInlineKeyboard(markup interface{}) (*tgbotapi.InlineKeyboardMarkup, error) {
	if markup == nil {
		return nil, nil
	}
	switch v := markup.(type) {
	case tgbotapi.InlineKeyboardMarkup:
		return &v, nil
	case *tgbotapi.InlineKeyboardMarkup:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported markup type %T", markup)
	}
}

func toBotMessage(msg tgbotapi.Message, markup interface{}) botport.BotMessage {
	payload := msg.Text
	if payload == "" {
		payload = msg.Caption
	}
	meta := metaFromMarkup(markup)
	return botport.BotMessage{
		ChatID:    chatIDFromMessage(msg),
		MessageID: msg.MessageID,
		Transport: "telegram",
		Payload:   payload,
		Meta:      meta,
	}
}

func metaFromMarkup(markup interface{}) map[string]string {
	if markup == nil {
		return nil
	}
	meta := map[string]string{
		"markup_type": fmt.Sprintf("%T", markup),
	}
	if keyboard, ok := extractInlineKeyboard(markup); ok {
		if raw, err := json.Marshal(keyboard); err == nil {
			meta["raw_markup"] = string(raw)
		}
	}
	return meta
}

func extractInlineKeyboard(markup interface{}) (*tgbotapi.InlineKeyboardMarkup, bool) {
	switch v := markup.(type) {
	case tgbotapi.InlineKeyboardMarkup:
		return &v, true
	case *tgbotapi.InlineKeyboardMarkup:
		return v, true
	default:
		return nil, false
	}
}

func chatIDFromMessage(msg tgbotapi.Message) int64 {
	if msg.Chat != nil {
		return msg.Chat.ID
	}
	return 0
}

func wrapContextError(op string, err error) error {
	if errors.Is(err, context.Canceled) {
		return &botport.BotError{Op: op, Code: "context_canceled", Wrapped: err}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &botport.BotError{Op: op, Code: "context_deadline", Wrapped: err}
	}
	return &botport.BotError{Op: op, Code: "context_error", Wrapped: err}
}

func wrapTelegramError(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return wrapContextError(op, err)
	}
	code, retry := classifyTelegramError(err)
	return &botport.BotError{
		Op:         op,
		Code:       code,
		RetryAfter: retry,
		Wrapped:    err,
	}
}

var retryAfterRegex = regexp.MustCompile(`(?i)retry after (\d+)`)

func classifyTelegramError(err error) (string, time.Duration) {
	if err == nil {
		return "unknown", 0
	}
	msg := err.Error()
	switch {
	case strings.Contains(strings.ToLower(msg), "message is not modified"):
		return "message_not_modified", 0
	case strings.Contains(strings.ToLower(msg), "too many requests"):
		return "rate_limited", extractRetryAfter(msg)
	case strings.Contains(strings.ToLower(msg), "bad request"):
		return "bad_request", 0
	case strings.Contains(strings.ToLower(msg), "forbidden"):
		return "forbidden", 0
	default:
		return "unknown", 0
	}
}

func extractRetryAfter(msg string) time.Duration {
	matches := retryAfterRegex.FindStringSubmatch(msg)
	if len(matches) != 2 {
		return 0
	}
	seconds, err := time.ParseDuration(matches[1] + "s")
	if err != nil {
		return 0
	}
	return seconds
}

func getBotErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var be *botport.BotError
	if errors.As(err, &be) {
		return be.Code
	}
	return ""
}
