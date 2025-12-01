package fsm

import (
	"context"
	"fmt"
	"github.com/dkalashnik/telegram-survey-bot/pkg/ports/botport"
	"github.com/dkalashnik/telegram-survey-bot/pkg/state"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/looplab/fsm"
)

func NewMainMenuFSM(initialState string) *fsm.FSM {

	callbacks := fsm.Callbacks{}

	events := fsm.Events{
		{Name: EventViewList, Src: []string{StateIdle}, Dst: StateViewingList},
		{Name: EventListNext, Src: []string{StateViewingList}, Dst: StateViewingList},
		{Name: EventListBack, Src: []string{StateViewingList}, Dst: StateViewingList},
		{Name: EventBackToIdle, Src: []string{StateViewingList}, Dst: StateIdle},
	}

	return fsm.NewFSM(initialState, events, callbacks)
}

func sendMainMenu(ctx context.Context, botPort botport.BotPort, userState *state.UserState) {
	log.Printf("Entering sendMainMenu for user %d", userState.UserID)
	recordCount := len(userState.Records)
	userName := userState.UserName
	userID := userState.UserID

	stats := fmt.Sprintf("👤 Имя: %s\n🆔 ID: %d\n📊 Кол-во записей: %d",
		userName, userID, recordCount)
	log.Printf("Stats: %s", stats)

	mainMenuKeyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(ButtonMainMenuShowRecord),
			tgbotapi.NewKeyboardButton(ButtonMainMenuFillRecord),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(ButtonMainMenuSendSelf),
			tgbotapi.NewKeyboardButton(ButtonMainMenuSendTherapist),
		),
	)

	_, err := botPort.SendMessage(ctx, userState.UserID, stats+"\n\nВыберите действие:", mainMenuKeyboard)
	if err != nil {
		log.Printf("[sendMainMenu] Error sending main menu for user %d: %v", userState.UserID, err)
	} else {
		log.Printf("[sendMainMenu] Main menu sent to user %d", userState.UserID)
	}
}

func viewLastRecordHandler(ctx context.Context, userState *state.UserState, botPort botport.BotPort, chatID int64) {
	var lastRecord *state.Record
	for i := len(userState.Records) - 1; i >= 0; i-- {
		if userState.Records[i].IsSaved {
			lastRecord = userState.Records[i]
			break
		}
	}

	if lastRecord == nil {
		_, _ = botPort.SendMessage(ctx, chatID, "У вас еще нет сохраненных записей.", nil)
		return
	}

	recordText := formatRecordForDisplay(lastRecord)
	status := "Сохранена"

	shareKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✉️ Поделиться", CallbackActionPrefix+ActionShareLast),
		),
	)

	msgText := fmt.Sprintf("📄 Последняя запись (Статус: %s):\n\n%s", status, recordText)
	_, err := botPort.SendMessage(ctx, chatID, msgText, shareKeyboard)
	if err != nil {
		log.Printf("[viewLastRecordHandler] Error sending last record for user %d: %v", chatID, err)
	}
}

func viewListHandler(ctx context.Context, userState *state.UserState, botPort botport.BotPort, chatID int64, messageID int) {
	const pageSize = 5

	offset := userState.ListOffset
	allRecords := make([]*state.Record, len(userState.Records))
	copy(allRecords, userState.Records)

	savedRecords := []*state.Record{}
	for _, r := range allRecords {
		if r.IsSaved {
			savedRecords = append(savedRecords, r)
		}
	}
	totalRecords := len(savedRecords)

	if totalRecords == 0 {
		text := "У вас еще нет сохраненных записей."
		var kbd interface{}
		if messageID != 0 {
			kbd = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
		}
		if messageID != 0 {
			_, _ = botPort.EditMessage(ctx, chatID, messageID, text, kbd.(*tgbotapi.InlineKeyboardMarkup))
		} else {
			_, _ = botPort.SendMessage(ctx, chatID, text, kbd)
		}

		if userState.MainMenuFSM.Current() == StateViewingList {
			err := userState.MainMenuFSM.Event(ctx, EventBackToIdle, userState, chatID)
			if err != nil {
				log.Printf("[viewListHandler] Error transitioning main FSM to idle for user %d: %v", chatID, err)
			}
		}
		return
	}

	start := offset
	end := offset + pageSize
	if start < 0 {
		start = 0
	}
	if start >= totalRecords {
		start = (totalRecords / pageSize) * pageSize
		if start == totalRecords {
			start = totalRecords - pageSize
			if start < 0 {
				start = 0
			}
		}
	}
	if end > totalRecords {
		end = totalRecords
	}

	pageRecords := []*state.Record{}
	if start < end {
		revStart := totalRecords - end
		revEnd := totalRecords - start
		if revStart < 0 {
			revStart = 0
		}
		pageRecords = savedRecords[revStart:revEnd]
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("🗂️ Список записей (%d - %d из %d):\n\n", start+1, end, totalRecords))

	if len(pageRecords) == 0 && totalRecords > 0 {
		builder.WriteString("Нет записей на этой странице.")
	} else {
		for i := len(pageRecords) - 1; i >= 0; i-- {
			r := pageRecords[i]
			builder.WriteString(fmt.Sprintf("📌 ID: ...%s (%s)\n", getLastNChars(r.ID, 6), r.CreatedAt.Format("02.01.06 15:04")))

			if name, ok := r.Data["name"]; ok && name != "" {
				builder.WriteString(fmt.Sprintf("   Имя: %s\n", truncateString(name, 25)))
			}
			if city, ok := r.Data["city"]; ok && city != "" {
				builder.WriteString(fmt.Sprintf("   Город: %s\n", truncateString(city, 25)))
			}
			builder.WriteString("---\n")
		}
	}

	hasPrev := start > 0
	hasNext := end < totalRecords
	keyboard := listNavigationKeyboard(hasPrev, hasNext)

	text := builder.String()
	if messageID != 0 {
		_, err := botPort.EditMessage(ctx, chatID, messageID, text, &keyboard)
		if err != nil && !strings.Contains(err.Error(), "message is not modified") {
			log.Printf("[viewListHandler] Error editing list for user %d: %v", chatID, err)
		}
	} else {
		_, err := botPort.SendMessage(ctx, chatID, text, keyboard)
		if err != nil {
			log.Printf("[viewListHandler] Error sending list for user %d: %v", chatID, err)
		}
	}
}

func formatRecordForDisplay(r *state.Record) string {
	if r == nil || r.Data == nil {
		return "Данные записи отсутствуют."
	}
	var sb strings.Builder

	if val, ok := r.Data["name"]; ok {
		sb.WriteString(fmt.Sprintf("Имя: %s\n", val))
	}
	if val, ok := r.Data["city"]; ok {
		sb.WriteString(fmt.Sprintf("Город: %s\n", val))
	}
	if val, ok := r.Data["age"]; ok {
		sb.WriteString(fmt.Sprintf("Возраст: %s\n", val))
	}
	if val, ok := r.Data["company"]; ok {
		sb.WriteString(fmt.Sprintf("Компания: %s\n", val))
	}
	if val, ok := r.Data["employment"]; ok {
		sb.WriteString(fmt.Sprintf("Занятость: %s\n", val))
	}
	if val, ok := r.Data["notes"]; ok {
		sb.WriteString(fmt.Sprintf("Заметки: %s\n", val))
	}

	text := sb.String()
	if text == "" {
		return "Нет заполненных данных."
	}
	return text
}

func listNavigationKeyboard(hasPrev, hasNext bool) tgbotapi.InlineKeyboardMarkup {
	row := []tgbotapi.InlineKeyboardButton{}
	if hasPrev {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", CallbackListNavPrefix+"back"))
	}
	if hasNext {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData("Вперед ➡️", CallbackListNavPrefix+"next"))
	}

	backRow := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬆️ В главное меню", CallbackListNavPrefix+"tomenu"),
	}

	if len(row) > 0 {
		return tgbotapi.NewInlineKeyboardMarkup(row, backRow)
	} else if len(backRow) > 0 {
		return tgbotapi.NewInlineKeyboardMarkup(backRow)
	}

	return tgbotapi.NewInlineKeyboardMarkup()
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
func getLastNChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
