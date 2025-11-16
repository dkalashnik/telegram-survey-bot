package fsm

import (
	"context"
	"fmt"
	"log"
	"strings"
	"telegramsurveylog/pkg/config"
	"telegramsurveylog/pkg/fsm/questions"
	"telegramsurveylog/pkg/ports/botport"
	"telegramsurveylog/pkg/state"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/looplab/fsm"
)

func NewRecordFSM(initialState string) *fsm.FSM {

	callbacks := fsm.Callbacks{
		"enter_" + StateSelectingSection:  enterSelectingSection,
		"enter_" + StateAnsweringQuestion: enterAnsweringQuestion,
		"enter_" + StateRecordIdle:        enterRecordIdle,
	}

	events := fsm.Events{
		{Name: EventStartRecord, Src: []string{StateRecordIdle}, Dst: StateSelectingSection},
		{Name: EventSelectSection, Src: []string{StateSelectingSection}, Dst: StateAnsweringQuestion},
		{Name: EventAnswerQuestion, Src: []string{StateAnsweringQuestion}, Dst: StateAnsweringQuestion},
		{Name: EventSectionComplete, Src: []string{StateAnsweringQuestion}, Dst: StateSelectingSection},

		{Name: EventCancelSection, Src: []string{StateAnsweringQuestion}, Dst: StateSelectingSection},
		{Name: EventSaveFullRecord, Src: []string{StateSelectingSection}, Dst: StateRecordIdle},
		{Name: EventExitToMainMenu, Src: []string{StateSelectingSection}, Dst: StateRecordIdle},
		{Name: EventForceExit, Src: []string{StateSelectingSection, StateAnsweringQuestion}, Dst: StateRecordIdle},
	}

	return fsm.NewFSM(initialState, events, callbacks)
}

func enterSelectingSection(ctx context.Context, e *fsm.Event) {
	log.Printf("[enterSelectingSection] START - Event: %s, Src: %s", e.Event, e.Src)

	if len(e.Args) < 4 {
		log.Printf("[enterSelectingSection] FATAL: Not enough arguments (got %d, expected at least 4)", len(e.Args))
		return
	}
	userState, okS := e.Args[0].(*state.UserState)
	botPort, okB := e.Args[1].(botport.BotPort)
	recordConfig, okC := e.Args[2].(*config.RecordConfig)
	chatID, okCh := e.Args[3].(int64)
	var messageID int
	if len(e.Args) > 4 {
		messageID, _ = e.Args[4].(int)
	}

	if !okS || userState == nil {
		log.Printf("[enterSelectingSection] FATAL: Failed to cast or nil UserState arg")
		return
	}
	if !okB || botPort == nil {
		log.Printf("[enterSelectingSection] FATAL: Failed to cast or nil BotPort arg")
		return
	}
	if !okC || recordConfig == nil {
		log.Printf("[enterSelectingSection] FATAL: Failed to cast or nil RecordConfig arg")
		return
	}
	if !okCh {
		log.Printf("[enterSelectingSection] FATAL: Failed to cast ChatID arg")
		return
	}

	userID := userState.UserID
	log.Printf("[enterSelectingSection] Args extracted successfully for User %d. messageID: %d", userID, messageID)

	if recordConfig.Sections == nil {
		log.Printf("[enterSelectingSection] Error: RecordConfig.Sections is nil for user %d", userID)
		logAndForceExit(e, "RecordConfig.Sections is nil")
		return
	}
	sections := recordConfig.Sections
	log.Printf("[enterSelectingSection] Config check passed for User %d. Number of sections: %d", userID, len(sections))

	currentRec := userState.CurrentRecord
	if currentRec == nil {
		log.Printf("[enterSelectingSection] Error: UserState.CurrentRecord is nil for user %d", userID)
		logAndForceExit(e, "UserState.CurrentRecord is nil")
		return
	}
	if currentRec.Data == nil {
		log.Printf("[enterSelectingSection] Error: UserState.CurrentRecord.Data is nil for user %d", userID)
		logAndForceExit(e, "UserState.CurrentRecord.Data is nil")
		return
	}
	recordData := currentRec.Data
	log.Printf("[enterSelectingSection] CurrentRecord check passed for User %d.", userID)

	prompt := "Выберите секцию для заполнения/редактирования или действие:"
	keyboard := tgbotapi.NewInlineKeyboardMarkup()
	log.Printf("[enterSelectingSection] Building keyboard for User %d...", userID)

	sectionIDs := getSortedSectionIDs(recordConfig.Sections)
	for _, sectionID := range sectionIDs {
		sectionConf := recordConfig.Sections[sectionID]
		hasData := sectionHasData(sectionConf, recordData)
		buttonText := sectionConf.Title
		if hasData {
			buttonText += " ✅"
		}

		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(buttonText, CallbackSectionPrefix+sectionID),
		)
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, row)
	}

	actionRow := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("💾 Сохранить запись", CallbackActionPrefix+ActionSaveRecord),
		tgbotapi.NewInlineKeyboardButtonData("⬆️ Выйти в меню", CallbackActionPrefix+ActionExitMenu),
	)
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, actionRow)

	var sentMsg botport.BotMessage
	var err error
	if messageID != 0 {
		sentMsg, err = botPort.EditMessage(ctx, chatID, messageID, prompt, &keyboard)
	} else {
		sentMsg, err = botPort.SendMessage(ctx, chatID, prompt, keyboard)
	}

	if err != nil {
		if !strings.Contains(err.Error(), "message is not modified") {
			log.Printf("[enterSelectingSection] Error sending/editing message for user %d: %v", chatID, err)
			_ = e.FSM.Event(ctx, EventForceExit, userState, botPort, recordConfig, chatID, 0, "error displaying section menu")
		} else {
			sentMsg.MessageID = messageID
		}
	}

	if err == nil || strings.Contains(err.Error(), "message is not modified") {
		userState.LastMessageID = sentMsg.MessageID
		userState.LastPrompt = toBotMessageFromPort(chatID, sentMsg.MessageID, prompt, &keyboard)
		log.Printf("[enterSelectingSection] Section selection menu shown/updated for user %d (MessageID: %d)", chatID, sentMsg.MessageID)
	}

	log.Printf("[enterSelectingSection] END - User %d", userID)
}

func askCurrentQuestion(ctx context.Context, userState *state.UserState, botPort botport.BotPort, recordConfig *config.RecordConfig, messageIDToEdit int) {
	log.Printf("[askCurrentQuestion] Preparing question for user %d, potentially editing message %d", userState.UserID, messageIDToEdit)

	sectionID := userState.CurrentSection
	qIndex := userState.CurrentQuestion
	lastMsgID := userState.LastMessageID

	sectionConf, okSec := recordConfig.Sections[sectionID]
	if !okSec {
		log.Printf("[askCurrentQuestion] Error: Section '%s' not found in config for user %d", sectionID, userState.UserID)
		_, _ = botPort.SendMessage(ctx, userState.UserID, "Ошибка конфигурации секции.", nil)
		return
	}

	if qIndex < 0 || qIndex >= len(sectionConf.Questions) {
		log.Printf("[askCurrentQuestion] Error: Invalid question index %d for section '%s' user %d", qIndex, sectionID, userState.UserID)
		_, _ = botPort.SendMessage(ctx, userState.UserID, "Ошибка навигации по вопросам.", nil)
		return
	}

	question := sectionConf.Questions[qIndex]
	strategy := questions.Get(question.Type)
	if strategy == nil {
		log.Printf("[askCurrentQuestion] Error: No strategy registered for type '%s'", question.Type)
		_, _ = botPort.SendMessage(ctx, userState.UserID, "Неизвестный тип вопроса. Попробуйте позже.", nil)
		return
	}

	renderCtx := questions.RenderContext{
		Bot:            botPort,
		LastPrompt:     userState.LastPrompt,
		ChatID:         userState.UserID,
		MessageID:      messageIDToEdit,
		UserState:      userState,
		Record:         userState.CurrentRecord,
		SectionID:      sectionID,
		Section:        sectionConf,
		Question:       question,
		CallbackPrefix: CallbackAnswerPrefix,
	}

	prompt, err := strategy.Render(renderCtx)
	if err != nil {
		log.Printf("[askCurrentQuestion] Error rendering question '%s': %v", question.ID, err)
		_, _ = botPort.SendMessage(ctx, userState.UserID, "Не удалось подготовить вопрос. Попробуйте позже.", nil)
		return
	}

	var keyboard *tgbotapi.InlineKeyboardMarkup
	if prompt.Keyboard != nil {
		keyboard = prompt.Keyboard
	} else {
		empty := tgbotapi.NewInlineKeyboardMarkup()
		keyboard = &empty
	}

	cancelRow := tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад к выбору секций", CallbackActionPrefix+ActionCancelSection))
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, cancelRow)

	var sentMsg botport.BotMessage
	isEdit := (messageIDToEdit != 0) && !prompt.ForceNew

	effectiveMessageID := messageIDToEdit
	if effectiveMessageID == 0 && lastMsgID != 0 && !prompt.ForceNew {
		effectiveMessageID = lastMsgID
		isEdit = true
		log.Printf("[askCurrentQuestion] Using LastMessageID (%d) for editing", effectiveMessageID)
	}

	if isEdit && effectiveMessageID != 0 {
		sentMsg, err = botPort.EditMessage(ctx, userState.UserID, effectiveMessageID, prompt.Text, keyboard)
	} else {
		sentMsg, err = botPort.SendMessage(ctx, userState.UserID, prompt.Text, keyboard)
	}

	if err != nil {
		if isEdit && botport.IsCode(err, "message_not_modified") {
			log.Printf("[askCurrentQuestion] Message %d not modified.", effectiveMessageID)
			sentMsg = botport.BotMessage{ChatID: userState.UserID, MessageID: effectiveMessageID, Transport: "telegram"}
		} else {
			log.Printf("[askCurrentQuestion] Error sending/editing question prompt for user %d (Q: %s): %v", userState.UserID, question.ID, err)
			return
		}
	} else {
		log.Printf("[askCurrentQuestion] Question '%s' sent/edited successfully. MessageID: %d", question.ID, sentMsg.MessageID)
	}

	userState.LastMessageID = sentMsg.MessageID
	userState.LastPrompt = sentMsg
	log.Printf("[askCurrentQuestion] Set LastMessageID to %d for user %d", sentMsg.MessageID, userState.UserID)
	log.Printf("[askCurrentQuestion] END - User %d", userState.UserID)
}

func enterAnsweringQuestion(ctx context.Context, e *fsm.Event) {
	log.Printf("[enterAnsweringQuestion] ****** ENTER CALLBACK START ****** - Event: %s, Src: %s", e.Event, e.Src)
	if len(e.Args) < 4 {
		log.Printf("[enterAnsweringQuestion] Error: not enough args")
		return
	}
	userState, okS := e.Args[0].(*state.UserState)
	botPort, okB := e.Args[1].(botport.BotPort)
	recordConfig, okC := e.Args[2].(*config.RecordConfig)
	messageIDFromEvent := 0
	if len(e.Args) > 4 {
		messageIDFromEvent, _ = e.Args[4].(int)
	}

	if !okS || !okB || !okC {
		log.Printf("[enterAnsweringQuestion] Error: invalid arg types")
		return
	}

	askCurrentQuestion(ctx, userState, botPort, recordConfig, messageIDFromEvent)
	log.Printf("[enterAnsweringQuestion] ****** ENTER CALLBACK END ****** - Event: %s, Src: %s", e.Event, e.Src)
}

func enterRecordIdle(ctx context.Context, e *fsm.Event) {
	if len(e.Args) < 4 {
		log.Printf("[enterRecordIdle] Error: not enough args for event %s", e.Event)
		return
	}
	userState, okS := e.Args[0].(*state.UserState)
	botPort, okB := e.Args[1].(botport.BotPort)
	chatID, okCh := e.Args[3].(int64)
	var messageID int
	if len(e.Args) > 4 {
		messageID, _ = e.Args[4].(int)
	}

	var failureReason string
	if len(e.Args) > 5 {
		failureReason, _ = e.Args[5].(string)
	}

	if !okS || !okB || !okCh {
		log.Printf("[enterRecordIdle] Error: Invalid argument types for event %s, user %d", e.Event, userState.UserID)
		sendMainMenu(ctx, botPort, userState)
		return
	}

	log.Printf("[enterRecordIdle] User %d entering RecordIdle state via event '%s'. MessageID: %d", chatID, e.Event, messageID)

	finalText := ""
	clearDraft := false
	saveRecord := false

	recordToFinalize := userState.CurrentRecord

	switch e.Event {
	case EventSaveFullRecord:
		if recordToFinalize != nil {
			recordToFinalize.IsSaved = true
			recordToFinalize.CreatedAt = time.Now()
			recordToFinalize.ID = fmt.Sprintf("%d-%d", userState.UserID, recordToFinalize.CreatedAt.UnixNano())
			finalText = "✅ Запись успешно сохранена!"
			saveRecord = true
			clearDraft = true
			log.Printf("[enterRecordIdle] Record marked for saving for user %d.", chatID)
		} else {
			finalText = "⚠️ Ошибка: Не найден черновик для сохранения."
			log.Printf("[enterRecordIdle] Error: CurrentRecord was nil when trying to save for user %d", chatID)
			clearDraft = true
		}
	case EventExitToMainMenu:
		finalText = "Выход из режима добавления. Черновик доступен для продолжения."
		clearDraft = false
		log.Printf("[enterRecordIdle] Exiting to main menu, draft kept for user %d.", chatID)
	case EventForceExit:
		finalText = fmt.Sprintf("⚠️ Произошла ошибка (%s). Ввод прерван. Черновик сохранен.", failureReason)
		clearDraft = false
		log.Printf("[enterRecordIdle] Force exiting record input for user %d. Reason: %s", chatID, failureReason)
	default:
		finalText = "Операция завершена."
		clearDraft = true
		log.Printf("[enterRecordIdle] Warning: RecordFSM entered idle state for user %d via unexpected event: %s", chatID, e.Event)
	}

	if saveRecord && recordToFinalize != nil {
		userState.Records = append(userState.Records, recordToFinalize)
		log.Printf("[enterRecordIdle] Record %s appended for user %d. Total records: %d", recordToFinalize.ID, chatID, len(userState.Records))
	}

	userState.CurrentSection = ""
	userState.CurrentQuestion = 0
	userState.LastMessageID = 0
	if clearDraft {
		userState.CurrentRecord = nil
		log.Printf("[enterRecordIdle] Draft cleared for user %d.", chatID)
	}

	if messageID != 0 {
		emptyKeyboard := &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
		_, err := botPort.EditMessage(ctx, chatID, messageID, finalText, emptyKeyboard)
		if err != nil && !strings.Contains(err.Error(), "message is not modified") {
			log.Printf("[enterRecordIdle] Error editing message %d for user %d: %v. Sending new message.", messageID, chatID, err)
			_, _ = botPort.SendMessage(ctx, chatID, finalText, nil)
		} else {
			log.Printf("[enterRecordIdle] Final status message shown/edited for user %d.", chatID)
		}
	} else {

		_, _ = botPort.SendMessage(ctx, chatID, finalText, nil)
	}

	sendMainMenu(ctx, botPort, userState)
}

func logAndForceExit(e *fsm.Event, errorMsg string) {
	log.Printf("Error in Record FSM callback: %s. Event: %s, Src: %s", errorMsg, e.Event, e.Src)
	if len(e.Args) >= 4 {
		userState, _ := e.Args[0].(*state.UserState)
		botPort, _ := e.Args[1].(botport.BotPort)
		recordConfig, _ := e.Args[2].(*config.RecordConfig)
		chatID, _ := e.Args[3].(int64)
		var messageID int
		if len(e.Args) > 4 {
			messageID, _ = e.Args[4].(int)
		}
		_ = e.FSM.Event(context.Background(), EventForceExit, userState, botPort, recordConfig, chatID, messageID, errorMsg)
	} else {
		log.Printf("Cannot trigger EventForceExit: not enough arguments in event %s", e.Event)
	}
}

func toBotMessageFromPort(chatID int64, messageID int, text string, markup interface{}) botport.BotMessage {
	meta := map[string]string{
		"markup_type": fmt.Sprintf("%T", markup),
	}
	return botport.BotMessage{
		ChatID:    chatID,
		MessageID: messageID,
		Transport: "telegram",
		Payload:   text,
		Meta:      meta,
	}
}

func sectionHasData(sectionConf config.SectionConfig, recordData map[string]string) bool {
	if recordData == nil {
		return false
	}
	for _, q := range sectionConf.Questions {
		if data, exists := recordData[q.StoreKey]; exists && data != "" {
			return true
		}
	}
	return false
}

func getSortedSectionIDs(sections map[string]config.SectionConfig) []string {
	keys := make([]string, 0, len(sections))
	for k := range sections {
		keys = append(keys, k)
	}

	return keys
}
