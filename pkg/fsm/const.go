package fsm

const (
	StateIdle        = "idle"
	StateViewingList = "viewingList"
)

const (
	StateRecordIdle        = "record_idle"
	StateSelectingSection  = "selecting_section"
	StateAnsweringQuestion = "answering_question"
)

const (
	EventStartAddRecord = "start_add_record"
	EventViewLast       = "view_last"
	EventViewList       = "view_list"
	EventListNext       = "list_next"
	EventListBack       = "list_back"
	EventBackToIdle     = "back_to_idle"
)

const (
	EventStartRecord     = "start_record"
	EventSelectSection   = "select_section"
	EventAnswerQuestion  = "answer_question"
	EventSectionComplete = "section_complete"
	EventCancelSection   = "cancel_section"
	EventSaveFullRecord  = "save_full_record"
	EventExitToMainMenu  = "exit_to_main_menu"
	EventForceExit       = "force_exit"
)

const (
	CallbackActionPrefix  = "action:"
	CallbackSectionPrefix = "section:"
	CallbackAnswerPrefix  = "answer:"
	CallbackListNavPrefix = "list_nav:"
)

const (
	ActionSaveRecord    = "save_record"
	ActionNewRecord     = "new_record"
	ActionExitMenu      = "exit_menu"
	ActionCancelSection = "cancel_section"
	ActionShareLast     = "share_last"
)

const (
	ButtonMainMenuFillRecord    = "Заполнить запись"
	ButtonMainMenuSendSelf      = "Отправить Себе"
	ButtonMainMenuSendTherapist = "Отправить Терапевту"
)
