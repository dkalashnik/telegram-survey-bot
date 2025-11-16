package fakeadapter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"telegramsurveylog/pkg/ports/botport"
)

// FakeAdapter implements botport.BotPort for headless tests.
type FakeAdapter struct {
	mu            sync.Mutex
	Calls         []Call
	NextMessageID int
	FailNext      map[string]error
}

// Call captures a bot operation invocation.
type Call struct {
	Op        string
	ChatID    int64
	MessageID int
	Text      string
	Markup    interface{}
	Callback  string
}

var _ botport.BotPort = (*FakeAdapter)(nil)

// SendMessage records a send operation and returns a synthetic BotMessage.
func (f *FakeAdapter) SendMessage(ctx context.Context, chatID int64, text string, markup interface{}) (botport.BotMessage, error) {
	if err := ctx.Err(); err != nil {
		return botport.BotMessage{}, wrapContextError("send_message", err)
	}
	if err := f.maybeFail("send_message"); err != nil {
		return botport.BotMessage{}, err
	}
	msgID := f.nextMessageID()
	f.record(Call{Op: "send_message", ChatID: chatID, MessageID: msgID, Text: text, Markup: markup})
	return f.botMessage(chatID, msgID, text), nil
}

// EditMessage records an edit operation and returns a synthetic BotMessage.
func (f *FakeAdapter) EditMessage(ctx context.Context, chatID int64, messageID int, text string, markup interface{}) (botport.BotMessage, error) {
	if err := ctx.Err(); err != nil {
		return botport.BotMessage{}, wrapContextError("edit_message", err)
	}
	if err := f.maybeFail("edit_message"); err != nil {
		return botport.BotMessage{}, err
	}
	if messageID == 0 {
		messageID = f.nextMessageID()
	}
	f.record(Call{Op: "edit_message", ChatID: chatID, MessageID: messageID, Text: text, Markup: markup})
	return f.botMessage(chatID, messageID, text), nil
}

// AnswerCallback records a callback acknowledgement.
func (f *FakeAdapter) AnswerCallback(ctx context.Context, callbackID string, text string) error {
	if err := ctx.Err(); err != nil {
		return wrapContextError("answer_callback", err)
	}
	if err := f.maybeFail("answer_callback"); err != nil {
		return err
	}
	f.record(Call{Op: "answer_callback", Callback: callbackID, Text: text})
	return nil
}

// Fail configures the next call for op to return err (wrapped as BotError if needed).
func (f *FakeAdapter) Fail(op string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FailNext == nil {
		f.FailNext = make(map[string]error)
	}
	f.FailNext[op] = err
}

// LastCall returns the most recent call for the given op.
func (f *FakeAdapter) LastCall(op string) *Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.Calls) - 1; i >= 0; i-- {
		if f.Calls[i].Op == op {
			c := f.Calls[i]
			return &c
		}
	}
	return nil
}

func (f *FakeAdapter) botMessage(chatID int64, messageID int, text string) botport.BotMessage {
	return botport.BotMessage{
		ChatID:    chatID,
		MessageID: messageID,
		Transport: "telegram",
		Payload:   text,
		Meta:      map[string]string{"fake": "true"},
	}
}

func (f *FakeAdapter) nextMessageID() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.NextMessageID == 0 {
		f.NextMessageID = 1
	}
	id := f.NextMessageID
	f.NextMessageID++
	return id
}

func (f *FakeAdapter) record(call Call) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, call)
}

func (f *FakeAdapter) maybeFail(op string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FailNext == nil {
		return nil
	}
	err, ok := f.FailNext[op]
	if !ok {
		return nil
	}
	delete(f.FailNext, op)
	if _, ok := err.(*botport.BotError); ok {
		return err
	}
	return &botport.BotError{Op: op, Code: "fake_error", Wrapped: err}
}

func wrapContextError(op string, err error) error {
	switch err {
	case context.Canceled:
		return &botport.BotError{Op: op, Code: "context_canceled", Wrapped: err}
	case context.DeadlineExceeded:
		return &botport.BotError{Op: op, Code: "context_deadline", Wrapped: err}
	default:
		return &botport.BotError{Op: op, Code: "context_error", Wrapped: err}
	}
}

// Helpers to script common BotError cases in tests.
func MessageNotModified(op string) *botport.BotError {
	return &botport.BotError{Op: op, Code: "message_not_modified"}
}

func RateLimited(op string, retry time.Duration) *botport.BotError {
	return &botport.BotError{Op: op, Code: "rate_limited", RetryAfter: retry, Wrapped: fmt.Errorf("rate limited")}
}
