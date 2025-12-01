package botport

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Package botport provides the outbound interface between the FSM and chat adapters.
// See PRPs/ai_docs/botport_hex_adapter.md for the architectural rationale and error semantics.

// BotMessage captures adapter-agnostic identifiers for previously sent messages.
type BotMessage struct {
	ChatID    int64
	MessageID int
	Transport string
	Payload   string
	Meta      map[string]string
}

// BotError wraps adapter failures with retry hints and normalized codes.
type BotError struct {
	Op         string
	Code       string
	RetryAfter time.Duration
	Wrapped    error
}

func (e *BotError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Wrapped != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Code, e.Wrapped)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Code)
}

// Unwrap exposes the underlying adapter error for errors.Is/As.
func (e *BotError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Wrapped
}

// NewBotError builds a BotError with the provided operation/code, preserving the wrapped error.
func NewBotError(op, code string, err error) *BotError {
	return &BotError{
		Op:      op,
		Code:    code,
		Wrapped: err,
	}
}

// IsCode determines whether err represents a BotError with the provided code.
func IsCode(err error, code string) bool {
	if err == nil {
		return false
	}
	var be *BotError
	if errors.As(err, &be) {
		return be != nil && be.Code == code
	}
	return false
}

// BotPort abstracts outbound message operations for adapters (Telegram, fake, etc.).
type BotPort interface {
	SendMessage(ctx context.Context, chatID int64, text string, markup interface{}) (BotMessage, error)
	EditMessage(ctx context.Context, chatID int64, messageID int, text string, markup interface{}) (BotMessage, error)
	AnswerCallback(ctx context.Context, callbackID string, text string) error
	DeleteMessage(ctx context.Context, chatID int64, messageID int) error
}
