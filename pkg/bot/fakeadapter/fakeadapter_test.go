package fakeadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dkalashnik/telegram-survey-bot/pkg/ports/botport"
)

func TestSendMessageRecordsCall(t *testing.T) {
	f := &FakeAdapter{}
	msg, err := f.SendMessage(context.Background(), 1, "hello", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.MessageID == 0 || msg.ChatID != 1 || msg.Transport != "telegram" || msg.Payload != "hello" {
		t.Fatalf("unexpected bot message: %+v", msg)
	}
	call := f.LastCall("send_message")
	if call == nil || call.Text != "hello" || call.ChatID != 1 {
		t.Fatalf("recorded call mismatch: %+v", call)
	}
}

func TestEditMessageUsesProvidedID(t *testing.T) {
	f := &FakeAdapter{}
	msg, err := f.EditMessage(context.Background(), 2, 99, "edit", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.MessageID != 99 {
		t.Fatalf("expected messageID 99, got %d", msg.MessageID)
	}
	call := f.LastCall("edit_message")
	if call == nil || call.MessageID != 99 {
		t.Fatalf("recorded call mismatch: %+v", call)
	}
}

func TestFailNextWrapsError(t *testing.T) {
	f := &FakeAdapter{}
	f.Fail("send_message", errors.New("boom"))
	_, err := f.SendMessage(context.Background(), 1, "x", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	var be *botport.BotError
	if !errors.As(err, &be) {
		t.Fatalf("expected BotError, got %T", err)
	}
	if be.Code != "fake_error" {
		t.Fatalf("expected fake_error, got %s", be.Code)
	}
}

func TestFailNextPassesThroughBotError(t *testing.T) {
	f := &FakeAdapter{}
	f.Fail("edit_message", MessageNotModified("edit_message"))
	_, err := f.EditMessage(context.Background(), 1, 2, "x", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	var be *botport.BotError
	if !errors.As(err, &be) {
		t.Fatalf("expected BotError, got %T", err)
	}
	if be.Code != "message_not_modified" {
		t.Fatalf("expected message_not_modified, got %s", be.Code)
	}
}

func TestRateLimitedHelperSetsRetryAfter(t *testing.T) {
	f := &FakeAdapter{}
	f.Fail("send_message", RateLimited("send_message", 2*time.Second))
	_, err := f.SendMessage(context.Background(), 1, "x", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	var be *botport.BotError
	if !errors.As(err, &be) {
		t.Fatalf("expected BotError, got %T", err)
	}
	if be.Code != "rate_limited" || be.RetryAfter != 2*time.Second {
		t.Fatalf("unexpected bot error: %+v", be)
	}
}

func TestAnswerCallbackRecorded(t *testing.T) {
	f := &FakeAdapter{}
	if err := f.AnswerCallback(context.Background(), "cbid", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := f.LastCall("answer_callback")
	if call == nil || call.Callback != "cbid" {
		t.Fatalf("expected callback recorded, got %+v", call)
	}
}
