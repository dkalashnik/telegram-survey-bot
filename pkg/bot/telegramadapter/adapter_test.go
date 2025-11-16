package telegramadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"telegramsurveylog/pkg/ports/botport"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestAdapterSendMessageSuccess(t *testing.T) {
	fc := &fakeClient{
		sendFn: func(chatID int64, text string, markup interface{}) (tgbotapi.Message, error) {
			return tgbotapi.Message{
				MessageID: 42,
				Text:      text,
				Chat:      &tgbotapi.Chat{ID: chatID},
			}, nil
		},
		editFn: func(chatID int64, messageID int, text string, markup *tgbotapi.InlineKeyboardMarkup) (tgbotapi.Message, error) {
			return tgbotapi.Message{MessageID: messageID, Text: text, Chat: &tgbotapi.Chat{ID: chatID}}, nil
		},
	}
	adapter, err := New(fc, testLogger{t})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("ok", "data"),
		),
	)

	msg, err := adapter.SendMessage(context.Background(), 7, "hello", keyboard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.ChatID != 7 || msg.MessageID != 42 {
		t.Fatalf("unexpected bot message: %+v", msg)
	}
	if msg.Transport != "telegram" {
		t.Fatalf("expected transport 'telegram', got %s", msg.Transport)
	}
	if msg.Payload != "hello" {
		t.Fatalf("expected payload 'hello', got %s", msg.Payload)
	}
	if msg.Meta["markup_type"] == "" {
		t.Fatalf("expected markup metadata to be set")
	}
	if msg.Meta["raw_markup"] == "" {
		t.Fatalf("expected raw markup to be serialized")
	}
}

func TestAdapterSendMessageWrapsRateLimitError(t *testing.T) {
	expectedErr := errors.New("Too Many Requests: retry after 3")
	fc := &fakeClient{
		sendFn: func(int64, string, interface{}) (tgbotapi.Message, error) {
			return tgbotapi.Message{}, expectedErr
		},
		editFn: func(int64, int, string, *tgbotapi.InlineKeyboardMarkup) (tgbotapi.Message, error) {
			return tgbotapi.Message{}, nil
		},
	}
	adapter, err := New(fc, testLogger{t})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = adapter.SendMessage(context.Background(), 1, "hi", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	var be *botport.BotError
	if !errors.As(err, &be) {
		t.Fatalf("expected BotError, got %T", err)
	}
	if be.Code != "rate_limited" {
		t.Fatalf("expected rate_limited code, got %s", be.Code)
	}
	if be.RetryAfter != 3*time.Second {
		t.Fatalf("expected RetryAfter=3s, got %v", be.RetryAfter)
	}
}

func TestAdapterEditMessageRejectsInvalidMarkup(t *testing.T) {
	fc := &fakeClient{
		sendFn: func(int64, string, interface{}) (tgbotapi.Message, error) {
			return tgbotapi.Message{}, nil
		},
		editFn: func(int64, int, string, *tgbotapi.InlineKeyboardMarkup) (tgbotapi.Message, error) {
			return tgbotapi.Message{}, nil
		},
	}
	adapter, err := New(fc, testLogger{t})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = adapter.EditMessage(context.Background(), 1, 2, "text", "bad markup")
	if err == nil {
		t.Fatalf("expected error")
	}
	var be *botport.BotError
	if !errors.As(err, &be) {
		t.Fatalf("expected BotError, got %T", err)
	}
	if be.Code != "bad_payload" {
		t.Fatalf("expected bad_payload, got %s", be.Code)
	}
}

type fakeClient struct {
	sendFn func(chatID int64, text string, markup interface{}) (tgbotapi.Message, error)
	editFn func(chatID int64, messageID int, text string, markup *tgbotapi.InlineKeyboardMarkup) (tgbotapi.Message, error)
	cbFn   func(callbackID string, text string) error
}

func (f *fakeClient) SendMessage(chatID int64, text string, markup interface{}) (tgbotapi.Message, error) {
	if f.sendFn == nil {
		return tgbotapi.Message{}, nil
	}
	return f.sendFn(chatID, text, markup)
}

func (f *fakeClient) EditMessageText(chatID int64, messageID int, text string, markup *tgbotapi.InlineKeyboardMarkup) (tgbotapi.Message, error) {
	if f.editFn == nil {
		return tgbotapi.Message{}, nil
	}
	return f.editFn(chatID, messageID, text, markup)
}

func (f *fakeClient) AnswerCallback(callbackID string, text string) error {
	if f.cbFn == nil {
		return nil
	}
	return f.cbFn(callbackID, text)
}

type testLogger struct {
	t *testing.T
}

func (l testLogger) Printf(format string, args ...any) {
	l.t.Logf(format, args...)
}
