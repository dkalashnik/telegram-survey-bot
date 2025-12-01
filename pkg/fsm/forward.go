package fsm

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"sort"
	"text/template"
	"time"

	"github.com/dkalashnik/telegram-survey-bot/pkg/config"
	"github.com/dkalashnik/telegram-survey-bot/pkg/ports/botport"
	"github.com/dkalashnik/telegram-survey-bot/pkg/state"
)

const (
	noAnswerPlaceholder = "no_answer"
)

type forwardQuestion struct {
	Prompt string
	Answer string
}

type forwardSection struct {
	Title     string
	Questions []forwardQuestion
}

type forwardPayload struct {
	UserID    int64
	UserName  string
	CreatedAt string
	Sections  []forwardSection
}

var forwardTpl = template.Must(template.New("forward").Parse(`Ответы пользователя {{.UserName}} (ID: {{.UserID}})
Дата записи: {{.CreatedAt}}
{{range .Sections}}## {{.Title}}
{{range .Questions}}- {{.Prompt}}:
  {{.Answer}}
{{end}}
{{end}}`))

func handleForwardAnsweredSections(ctx context.Context, userState *state.UserState, botPort botport.BotPort, recordConfig *config.RecordConfig, chatID int64) {
	targetUserID := config.GetTargetUserID()
	handleForwardToTarget(ctx, userState, botPort, recordConfig, chatID, targetUserID, false)
}

func handleForwardToTarget(ctx context.Context, userState *state.UserState, botPort botport.BotPort, recordConfig *config.RecordConfig, chatID int64, targetUserID int64, clearOnSuccess bool) {
	forwardWithTarget(ctx, userState, botPort, recordConfig, chatID, targetUserID, clearOnSuccess, true, func(id int64) string {
		return fmt.Sprintf("Ответы отправлены на ID %d.", id)
	})
}

func handleForwardToSelf(ctx context.Context, userState *state.UserState, botPort botport.BotPort, recordConfig *config.RecordConfig, chatID int64) {
	forwardWithTarget(ctx, userState, botPort, recordConfig, chatID, chatID, false, false, func(id int64) string {
		return "Ответы отправлены вам в этот чат."
	})
}

func forwardWithTarget(ctx context.Context, userState *state.UserState, botPort botport.BotPort, recordConfig *config.RecordConfig, chatID int64, targetUserID int64, clearOnSuccess bool, requireConfigured bool, successText func(int64) string) {
	record := selectRecordForForward(userState)
	if record == nil {
		_, _ = botPort.SendMessage(ctx, chatID, "Нет ответов для отправки.", nil)
		return
	}

	if requireConfigured && targetUserID == 0 {
		log.Printf("[handleForwardAnsweredSections] TARGET_USER_ID is not configured")
		_, _ = botPort.SendMessage(ctx, chatID, "Не настроен TARGET_USER_ID, отправка недоступна.", nil)
		return
	}

	payload := buildForwardPayload(recordConfig, record, userState)
	text, err := renderForwardMessage(payload)
	if err != nil {
		log.Printf("[handleForwardAnsweredSections] render error for user %d: %v", userState.UserID, err)
		_, _ = botPort.SendMessage(ctx, chatID, "Не удалось сформировать сообщение для отправки.", nil)
		return
	}

	if len(text) == 0 {
		log.Printf("[handleForwardAnsweredSections] empty rendered text for user %d", userState.UserID)
		_, _ = botPort.SendMessage(ctx, chatID, "Нет данных для отправки.", nil)
		return
	}

	log.Printf("[handleForwardAnsweredSections] forwarding record %s for user %d to target %d (clear=%t)", record.ID, userState.UserID, targetUserID, clearOnSuccess)
	_, err = botPort.SendMessage(ctx, targetUserID, text, nil)
	if err != nil {
		log.Printf("[handleForwardAnsweredSections] forward error for user %d to %d: %v", userState.UserID, targetUserID, err)
		_, _ = botPort.SendMessage(ctx, chatID, "Не удалось отправить ответы, попробуйте позже.", nil)
		return
	}

	if clearOnSuccess {
		if targetUserID == chatID {
			log.Printf("[handleForwardAnsweredSections] TARGET_USER_ID %d matches requester chat %d; check configuration if a different recipient was expected", targetUserID, chatID)
		}

		clearUserAnswers(userState, record)
	}
	confirmation := successText(targetUserID)
	_, _ = botPort.SendMessage(ctx, chatID, confirmation, nil)
}

// selectRecordForForward chooses the most recent saved record if present; otherwise falls back to the current draft.
// Only the selected record is cleared after a successful forward; other saved records remain intact.
func selectRecordForForward(userState *state.UserState) *state.Record {
	for i := len(userState.Records) - 1; i >= 0; i-- {
		if userState.Records[i] != nil && userState.Records[i].IsSaved {
			return userState.Records[i]
		}
	}
	if userState.CurrentRecord != nil {
		return userState.CurrentRecord
	}
	return nil
}

func buildForwardPayload(recordConfig *config.RecordConfig, record *state.Record, userState *state.UserState) forwardPayload {
	sections := make([]forwardSection, 0, len(recordConfig.Sections))
	sectionIDs := make([]string, 0, len(recordConfig.Sections))
	for id := range recordConfig.Sections {
		sectionIDs = append(sectionIDs, id)
	}
	sort.Strings(sectionIDs)

	for _, sectionID := range sectionIDs {
		sectionConf := recordConfig.Sections[sectionID]
		qs := make([]forwardQuestion, 0, len(sectionConf.Questions))
		for _, q := range sectionConf.Questions {
			answer := ""
			if record != nil && record.Data != nil {
				answer = record.Data[q.StoreKey]
			}
			if answer == "" {
				answer = noAnswerPlaceholder
			}
			qs = append(qs, forwardQuestion{
				Prompt: q.Prompt,
				Answer: answer,
			})
		}
		sections = append(sections, forwardSection{
			Title:     sectionConf.Title,
			Questions: qs,
		})
	}

	created := record.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}

	return forwardPayload{
		UserID:    userState.UserID,
		UserName:  userState.UserName,
		CreatedAt: created.Format("02.01.2006 15:04"),
		Sections:  sections,
	}
}

func renderForwardMessage(payload forwardPayload) (string, error) {
	var buf bytes.Buffer
	if err := forwardTpl.Execute(&buf, payload); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func clearUserAnswers(userState *state.UserState, forwarded *state.Record) {
	// Preserve other saved records; drop only the forwarded record/draft.
	filtered := make([]*state.Record, 0, len(userState.Records))
	for _, r := range userState.Records {
		if r == nil || r == forwarded {
			continue
		}
		filtered = append(filtered, r)
	}
	userState.Records = filtered
	if userState.CurrentRecord == forwarded {
		userState.CurrentRecord = nil
	}
	userState.CurrentSection = ""
	userState.CurrentQuestion = 0
	userState.LastMessageID = 0
	userState.LastPrompt = botport.BotMessage{}
}
