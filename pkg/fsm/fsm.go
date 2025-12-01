package fsm

import (
	"context"
	"fmt"
	"github.com/dkalashnik/telegram-survey-bot/pkg/config"
	"github.com/dkalashnik/telegram-survey-bot/pkg/fsm/questions"
	"github.com/dkalashnik/telegram-survey-bot/pkg/ports/botport"
	"github.com/dkalashnik/telegram-survey-bot/pkg/state"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func HandleUpdate(ctx context.Context, update tgbotapi.Update, botPort botport.BotPort, recordConfig *config.RecordConfig, store *state.Store) {

	var userID int64
	var chatID int64
	var userName string
	var from *tgbotapi.User

	if update.Message != nil {
		if update.Message.From == nil {
			log.Printf("Warning: Received message with nil From field")
			return
		}
		from = update.Message.From
		chatID = update.Message.Chat.ID
	} else if update.CallbackQuery != nil {
		if update.CallbackQuery.From == nil {
			log.Printf("Warning: Received callback with nil From field")
			return
		}
		from = update.CallbackQuery.From
		if update.CallbackQuery.Message == nil || update.CallbackQuery.Message.Chat == nil {
			log.Printf("Warning: Received callback query with nil Message or Chat field")
			return
		}
		chatID = update.CallbackQuery.Message.Chat.ID
	} else {

		log.Printf("Ignoring update type: %v", update)
		return
	}

	userID = from.ID
	userName = from.FirstName
	if from.LastName != "" {
		userName += " " + from.LastName
	}

	userState := store.GetOrCreateUserState(userID, userName)
	if userState == nil {
		log.Printf("Error: Failed to get or create user state for user %d", userID)

		if chatID != 0 {
			_, _ = botPort.SendMessage(ctx, chatID, "Произошла внутренняя ошибка. Пожалуйста, попробуйте позже или обратитесь к администратору.", nil)
		}
		return
	}

	userState.Mu.Lock()
	defer userState.Mu.Unlock()

	if update.Message != nil {
		handleMessage(ctx, update.Message, userState, botPort, recordConfig)
	} else if update.CallbackQuery != nil {
		handleCallbackQuery(ctx, update.CallbackQuery, userState, botPort, recordConfig)
	}
}

func handleMessage(ctx context.Context, message *tgbotapi.Message, userState *state.UserState, botPort botport.BotPort, recordConfig *config.RecordConfig) {
	chatID := message.Chat.ID
	text := message.Text

	if message.IsCommand() {
		switch message.Command() {
		case "start":
			chatID := message.Chat.ID

			if userState.RecordFSM.Current() != StateRecordIdle {
				log.Printf("User %d used /start, resetting RecordFSM from %s to idle", userState.UserID, userState.RecordFSM.Current())

				lastMsgID := userState.LastMessageID

				err := userState.RecordFSM.Event(ctx, EventForceExit, userState, botPort, recordConfig, chatID, lastMsgID, "command /start used")

				if err != nil {

					log.Printf("Error triggering EventForceExit via /start for user %d: %v. Attempting SetState.", userState.UserID, err)

					userState.RecordFSM.SetState(StateRecordIdle)

					log.Printf("Manually cleaning up state and sending main menu after SetState fallback for user %d", userState.UserID)

					userState.CurrentSection = ""
					userState.CurrentQuestion = 0
					userState.LastMessageID = 0

					sendMainMenu(ctx, botPort, userState)

				}

			} else {

				log.Printf("User %d used /start while already in idle state. Sending main menu.", userState.UserID)
				sendMainMenu(ctx, botPort, userState)
			}
			return

		default:
			_, _ = botPort.SendMessage(ctx, chatID, "Неизвестная команда.", nil)
			return
		}
	}

	mainState := userState.MainMenuFSM.Current()
	recordState := userState.RecordFSM.Current()

	if recordState == StateAnsweringQuestion {
		sectionConf, question, err := resolveCurrentQuestion(recordConfig, userState)
		if err != nil {
			log.Printf("[handleMessage] %v", err)
			_ = userState.RecordFSM.Event(ctx, EventForceExit, userState, botPort, recordConfig, chatID, userState.LastMessageID, "invalid state/config for text answer")
			return
		}

		strategy := questions.Get(question.Type)
		if strategy == nil {
			log.Printf("[handleMessage] Error: No strategy for question type '%s'", question.Type)
			_ = userState.RecordFSM.Event(ctx, EventForceExit, userState, botPort, recordConfig, chatID, userState.LastMessageID, "missing question strategy")
			return
		}

		answerCtx := buildAnswerContext(userState, sectionConf, question, chatID, userState.LastMessageID, "", userState.LastPrompt, botPort)
		result, err := strategy.HandleAnswer(answerCtx, questions.AnswerInput{
			Source:    questions.InputSourceText,
			Text:      text,
			MessageID: userState.LastMessageID,
		})
		if err != nil {
			log.Printf("[handleMessage] Error processing answer for user %d: %v", userState.UserID, err)
			_ = userState.RecordFSM.Event(ctx, EventForceExit, userState, botPort, recordConfig, chatID, userState.LastMessageID, "strategy failed while handling answer")
			return
		}

		handleAnswerResult(ctx, result, userState, botPort, recordConfig, userState.LastMessageID)
		return
	}

	if mainState == StateIdle && recordState == StateRecordIdle {
		switch text {
		case ButtonMainMenuFillRecord:
			log.Printf("[handleMessage] User %d initiated record creation", userState.UserID)

			startOrResumeRecordCreation(ctx, userState, botPort, recordConfig, chatID)

			hideKeyboard(ctx, botPort, chatID, "Начинаем ввод/продолжение записи...")

		case ButtonMainMenuShowRecord:
			log.Printf("[handleMessage] User %d requested last record view", userState.UserID)
			viewLastRecordHandler(ctx, userState, botPort, recordConfig, chatID)

		case ButtonMainMenuSendSelf:
			log.Printf("[handleMessage] User %d requested forward to self", userState.UserID)
			handleForwardToSelf(ctx, userState, botPort, recordConfig, chatID)

		case ButtonMainMenuSendTherapist:
			log.Printf("[handleMessage] User %d requested forward to therapist", userState.UserID)
			handleForwardAnsweredSections(ctx, userState, botPort, recordConfig, chatID)

		default:

		}
		return
	}

	_, _ = botPort.SendMessage(ctx, chatID, "Пожалуйста, используйте предложенные кнопки или завершите текущее действие.", nil)
}

func handleCallbackQuery(ctx context.Context, query *tgbotapi.CallbackQuery, userState *state.UserState, botPort botport.BotPort, recordConfig *config.RecordConfig) {
	chatID := query.Message.Chat.ID
	messageID := query.Message.MessageID
	data := query.Data

	err := botPort.AnswerCallback(ctx, query.ID, "")
	if err != nil {
		log.Printf("[handleCallbackQuery] Error answering callback %s for user %d: %v", query.ID, userState.UserID, err)

	}

	parts := strings.SplitN(data, ":", 2)
	prefix := parts[0] + ":"
	value := ""
	if len(parts) > 1 {
		value = parts[1]
	}

	log.Printf("[handleCallbackQuery] Received callback: Prefix='%s', Value='%s', UserID=%d, State=%s/%s",
		prefix, value, userState.UserID, userState.MainMenuFSM.Current(), userState.RecordFSM.Current())

	recordState := userState.RecordFSM.Current()
	mainState := userState.MainMenuFSM.Current()

	switch prefix {
	case CallbackAnswerPrefix:
		if recordState == StateAnsweringQuestion {

			answerParts := strings.SplitN(value, ":", 2)
			if len(answerParts) != 2 {
				log.Printf("[handleCallbackQuery] Error: Invalid answer callback data format '%s' for user %d", value, userState.UserID)
				return
			}
			questionID := answerParts[0]
			optionValue := answerParts[1]

			currentQID := ""
			currentSectionConf, okSec := recordConfig.Sections[userState.CurrentSection]
			if okSec && userState.CurrentQuestion >= 0 && userState.CurrentQuestion < len(currentSectionConf.Questions) {
				currentQID = currentSectionConf.Questions[userState.CurrentQuestion].ID
			}

			if currentQID == questionID {
				log.Printf("[handleCallbackQuery] Processing button answer for user %d (Q: %s, Value: %s)", userState.UserID, questionID, optionValue)

				question := currentSectionConf.Questions[userState.CurrentQuestion]
				strategy := questions.Get(question.Type)
				if strategy == nil {
					log.Printf("[handleCallbackQuery] Error: No strategy for question type '%s'", question.Type)
					_ = userState.RecordFSM.Event(ctx, EventForceExit, userState, botPort, recordConfig, chatID, messageID, "missing question strategy")
					return
				}

				answerCtx := buildAnswerContext(userState, currentSectionConf, question, chatID, messageID, query.ID, userState.LastPrompt, botPort)
				result, err := strategy.HandleAnswer(answerCtx, questions.AnswerInput{
					Source:       questions.InputSourceCallback,
					CallbackData: optionValue,
					MessageID:    messageID,
				})
				if err != nil {
					log.Printf("[handleCallbackQuery] Error processing callback answer for user %d: %v", userState.UserID, err)
					_ = userState.RecordFSM.Event(ctx, EventForceExit, userState, botPort, recordConfig, chatID, messageID, "strategy failed while handling callback")
					return
				}

				handleAnswerResult(ctx, result, userState, botPort, recordConfig, messageID)
				return
			} else {
				log.Printf("[handleCallbackQuery] Warning: Received answer for question '%s', but current question is '%s' for user %d. Ignoring.", questionID, currentQID, userState.UserID)
				_ = botPort.AnswerCallback(ctx, query.ID, "⚠️ Ответ на предыдущий вопрос?")
				return
			}

		} else {
			log.Printf("[handleCallbackQuery] Warning: Received answer callback from user %d but not in AnsweringQuestion state (%s)", userState.UserID, recordState)
			return
		}

	case CallbackSectionPrefix:
		if recordState == StateSelectingSection {
			sectionID := value
			log.Printf("[handleCallbackQuery] User %d selected section '%s'", userState.UserID, sectionID)

			userState.CurrentSection = sectionID
			userState.CurrentQuestion = 0

			err := userState.RecordFSM.Event(ctx, EventSelectSection, userState, botPort, recordConfig, chatID, messageID)
			if err != nil {
				log.Printf("[handleCallbackQuery] Error triggering EventSelectSection for user %d: %v", userState.UserID, err)

				_ = userState.RecordFSM.Event(ctx, EventForceExit, userState, botPort, recordConfig, chatID, messageID, "failed to select section")
			}
		} else {
			log.Printf("[handleCallbackQuery] Warning: Received section selection callback from user %d but not in SelectingSection state (%s)", userState.UserID, recordState)
		}
		return

	case CallbackActionPrefix:
		actionName := value
		switch actionName {
		case ActionCancelSection:
			if recordState == StateAnsweringQuestion {
				log.Printf("[handleCallbackQuery] User %d cancelled section input", userState.UserID)
				err := userState.RecordFSM.Event(ctx, EventCancelSection, userState, botPort, recordConfig, chatID, messageID)
				if err != nil {
					log.Printf("[handleCallbackQuery] Error triggering EventCancelSection for user %d: %v", userState.UserID, err)
				}
			}
		case ActionSaveRecord:
			if recordState == StateSelectingSection {
				log.Printf("[handleCallbackQuery] User %d requested save record", userState.UserID)
				err := userState.RecordFSM.Event(ctx, EventSaveFullRecord, userState, botPort, recordConfig, chatID, messageID)
				if err != nil {
					log.Printf("[handleCallbackQuery] Error triggering EventSaveFullRecord for user %d: %v", userState.UserID, err)
				}
			}
		case ActionNewRecord:
			if recordState == StateSelectingSection {
				log.Printf("[handleCallbackQuery] User %d requested new record", userState.UserID)
				resetCurrentRecord(ctx, userState, botPort, recordConfig, chatID, messageID)
			}
		case ActionExitMenu:
			if recordState == StateSelectingSection {
				log.Printf("[handleCallbackQuery] User %d requested exit to menu", userState.UserID)
				err := userState.RecordFSM.Event(ctx, EventExitToMainMenu, userState, botPort, recordConfig, chatID, messageID)
				if err != nil {
					log.Printf("[handleCallbackQuery] Error triggering EventExitToMainMenu for user %d: %v", userState.UserID, err)
				}
			}
		case ActionShareLast:
			log.Printf("[handleCallbackQuery] User %d requested share last record", userState.UserID)
			handleShareLastRecord(ctx, userState, botPort, recordConfig, chatID)

		default:
			log.Printf("[handleCallbackQuery] Unknown action '%s' from user %d", actionName, userState.UserID)
		}
		return

	case CallbackListNavPrefix:
		if mainState == StateViewingList {
			navAction := value
			switch navAction {
			case "next":
				userState.ListOffset += 5
				log.Printf("[handleCallbackQuery] User %d requested next list page (offset %d)", userState.UserID, userState.ListOffset)

				viewListHandler(ctx, userState, botPort, chatID, messageID)

			case "back":
				newOffset := userState.ListOffset - 5
				if newOffset < 0 {
					newOffset = 0
				}
				userState.ListOffset = newOffset
				log.Printf("[handleCallbackQuery] User %d requested previous list page (offset %d)", userState.UserID, userState.ListOffset)

				viewListHandler(ctx, userState, botPort, chatID, messageID)

			case "tomenu":
				log.Printf("[handleCallbackQuery] User %d requested back to menu from list", userState.UserID)

				err := userState.MainMenuFSM.Event(ctx, EventBackToIdle, userState, botPort, recordConfig, chatID, messageID)
				if err != nil {
					log.Printf("[handleCallbackQuery] Error triggering EventBackToIdle for user %d: %v", userState.UserID, err)
				}

				emptyKeyboard := &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
				_, errEdit := botPort.EditMessage(ctx, chatID, messageID, query.Message.Text, emptyKeyboard)
				if errEdit != nil && !strings.Contains(errEdit.Error(), "message is not modified") {
					log.Printf("[handleCallbackQuery] Error removing inline keyboard from list message %d: %v", messageID, errEdit)
				}

				sendMainMenu(ctx, botPort, userState)

			default:
				log.Printf("[handleCallbackQuery] Unknown list navigation action '%s' from user %d", navAction, userState.UserID)
			}
		} else {
			log.Printf("[handleCallbackQuery] Warning: Received list navigation callback from user %d but not in ViewingList state (%s)", userState.UserID, mainState)

			_ = botPort.AnswerCallback(ctx, query.ID, "Действие недоступно.")
		}
		return

	default:
		log.Printf("[handleCallbackQuery] Unknown callback prefix '%s' from user %d", prefix, userState.UserID)
	}
}

func processAnswer(ctx context.Context, userState *state.UserState, botPort botport.BotPort, recordConfig *config.RecordConfig, messageID int) {

	sectionID := userState.CurrentSection
	qIndex := userState.CurrentQuestion
	sectionConf, okSec := recordConfig.Sections[sectionID]
	if !okSec || qIndex < 0 || qIndex >= len(sectionConf.Questions) {
		log.Printf("[processAnswer] Error: Invalid state/config for user %d (section %s, qIdx %d)", userState.UserID, sectionID, qIndex)
		_ = userState.RecordFSM.Event(ctx, EventForceExit, userState, botPort, recordConfig, userState.UserID, messageID, "invalid state/config in processAnswer")
		return
	}
	nextQIndex := qIndex + 1
	var nextEvent string
	if nextQIndex < len(sectionConf.Questions) {

		userState.CurrentQuestion = nextQIndex
		nextEvent = EventAnswerQuestion
		log.Printf("[processAnswer] Next question for user %d (Index: %d)", userState.UserID, nextQIndex)
	} else {

		userState.CurrentQuestion = 0
		userState.CurrentSection = ""
		nextEvent = EventSectionComplete
		log.Printf("[processAnswer] Section complete for user %d", userState.UserID)
	}

	log.Printf("[processAnswer] Triggering FSM event '%s' for user %d", nextEvent, userState.UserID)
	err := userState.RecordFSM.Event(ctx, nextEvent, userState, botPort, recordConfig, userState.UserID, messageID)
	if err != nil {
		if isNoTransitionError(err) {

			log.Printf("[processAnswer] FSM self-transition refused (expected for %s). Manually asking next question for user %d.", nextEvent, userState.UserID)

			askCurrentQuestion(ctx, userState, botPort, recordConfig, messageID)
		} else {

			log.Printf("[processAnswer] REAL Error triggering event '%s' for user %d: %v", nextEvent, userState.UserID, err)

			_, _ = botPort.SendMessage(ctx, userState.UserID, "Произошла внутренняя ошибка FSM.", nil)

		}
	} else {
		log.Printf("[processAnswer] Successfully triggered FSM event '%s' (transition occurred) for user %d", nextEvent, userState.UserID)
	}
	log.Printf("[processAnswer] END - User %d", userState.UserID)
}

func resolveCurrentQuestion(recordConfig *config.RecordConfig, userState *state.UserState) (config.SectionConfig, config.QuestionConfig, error) {
	sectionID := userState.CurrentSection
	qIndex := userState.CurrentQuestion
	sectionConf, okSec := recordConfig.Sections[sectionID]
	if !okSec || qIndex < 0 || qIndex >= len(sectionConf.Questions) {
		return config.SectionConfig{}, config.QuestionConfig{}, fmt.Errorf("invalid state/config when resolving question (section %s idx %d)", sectionID, qIndex)
	}
	return sectionConf, sectionConf.Questions[qIndex], nil
}

func buildAnswerContext(userState *state.UserState, sectionConf config.SectionConfig, question config.QuestionConfig, chatID int64, messageID int, callbackID string, lastPrompt botport.BotMessage, botPort botport.BotPort) questions.AnswerContext {
	return questions.AnswerContext{
		RenderContext: questions.RenderContext{
			Bot:            botPort,
			LastPrompt:     lastPrompt,
			ChatID:         chatID,
			MessageID:      messageID,
			UserState:      userState,
			Record:         userState.CurrentRecord,
			SectionID:      userState.CurrentSection,
			Section:        sectionConf,
			Question:       question,
			CallbackPrefix: CallbackAnswerPrefix,
		},
		Message:    lastPrompt,
		CallbackID: callbackID,
	}
}

func handleAnswerResult(ctx context.Context, result questions.AnswerResult, userState *state.UserState, botPort botport.BotPort, recordConfig *config.RecordConfig, messageID int) {
	if result.Feedback != "" {
		_, _ = botPort.SendMessage(ctx, userState.UserID, result.Feedback, nil)
	}

	if result.Repeat && !result.Advance {
		askCurrentQuestion(ctx, userState, botPort, recordConfig, messageID)
		return
	}

	if result.Advance {
		processAnswer(ctx, userState, botPort, recordConfig, messageID)
	}
}

func startOrResumeRecordCreation(ctx context.Context, userState *state.UserState, botPort botport.BotPort, recordConfig *config.RecordConfig, chatID int64) {

	if userState.CurrentRecord == nil {
		log.Printf("[startOrResumeRecordCreation] User %d starting new record.", userState.UserID)
		userState.CurrentRecord = state.NewRecord()
	} else {
		log.Printf("[startOrResumeRecordCreation] User %d resuming existing draft.", userState.UserID)

	}

	userState.CurrentSection = ""
	userState.CurrentQuestion = 0

	err := userState.RecordFSM.Event(ctx, EventStartRecord, userState, botPort, recordConfig, chatID, 0)
	if err != nil {
		log.Printf("[startOrResumeRecordCreation] Error triggering EventStartRecord for user %d: %v", userState.UserID, err)

		_, _ = botPort.SendMessage(ctx, chatID, "Не удалось начать ввод записи. Попробуйте позже.", nil)

		if userState.RecordFSM.Current() != StateRecordIdle {
			userState.RecordFSM.SetState(StateRecordIdle)
		}
	}

}

func hideKeyboard(ctx context.Context, botPort botport.BotPort, chatID int64, text string) {

	hideMsg := tgbotapi.NewMessage(chatID, text)
	hideMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	if _, err := botPort.SendMessage(ctx, chatID, text, hideMsg.ReplyMarkup); err != nil {
		log.Printf("[hideKeyboard] Error sending keyboard removal message for user %d: %v", chatID, err)
	} else {
		log.Printf("[hideKeyboard] Reply keyboard removal command sent to user %d.", chatID)
	}
}

func handleShareLastRecord(ctx context.Context, userState *state.UserState, botPort botport.BotPort, recordConfig *config.RecordConfig, chatID int64) {

	var lastRecord *state.Record
	for i := len(userState.Records) - 1; i >= 0; i-- {
		if userState.Records[i].IsSaved {
			lastRecord = userState.Records[i]
			break
		}
	}

	if lastRecord == nil {
		_, _ = botPort.SendMessage(ctx, chatID, "Нет сохраненных записей для пересылки.", nil)
		return
	}
	payload := buildForwardPayload(recordConfig, lastRecord, userState)
	shareText, err := renderForwardMessage(payload)
	if err != nil {
		log.Printf("[handleShareLastRecord] render error for user %d: %v", userState.UserID, err)
		_, _ = botPort.SendMessage(ctx, chatID, "Не удалось подготовить запись для отправки.", nil)
		return
	}
	_, _ = botPort.SendMessage(ctx, chatID, fmt.Sprintf("Чтобы поделиться, скопируйте текст ниже:\n\n---\n%s\n---", shareText), nil)
}

func resetCurrentRecord(ctx context.Context, userState *state.UserState, botPort botport.BotPort, recordConfig *config.RecordConfig, chatID int64, messageID int) {
	userState.CurrentRecord = state.NewRecord()
	userState.CurrentSection = ""
	userState.CurrentQuestion = 0
	showSectionSelectionMenu(ctx, userState, botPort, recordConfig, chatID, messageID, userState.CurrentRecord.Data, nil)
}
