package fsm

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/dkalashnik/telegram-survey-bot/pkg/fsm/questions"
	"github.com/dkalashnik/telegram-survey-bot/pkg/ports/botport"
)

// deleteUserTextMessage removes user messages for text-type answers when enabled.
func deleteUserTextMessage(ctx context.Context, botPort botport.BotPort, chatID int64, messageID int, questionType string) {
	if messageID == 0 {
		return
	}
	if !deleteEnabled() {
		return
	}
	if strings.ToLower(questionType) != questions.TypeText {
		return
	}
	if err := botPort.DeleteMessage(ctx, chatID, messageID); err != nil {
		log.Printf("[deleteUserTextMessage] failed to delete message %d for chat %d: %v", messageID, chatID, err)
	}
}

func deleteEnabled() bool {
	return strings.EqualFold(os.Getenv("DELETE_USER_MESSAGES"), "true")
}
