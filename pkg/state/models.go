package state

import (
	"sync"
	"time"

	"telegramsurveylog/pkg/ports/botport"

	"github.com/looplab/fsm"
)

type Record struct {
	ID        string
	Data      map[string]string
	IsSaved   bool
	CreatedAt time.Time
}

type UserState struct {
	UserID          int64
	UserName        string
	Records         []*Record
	MainMenuFSM     *fsm.FSM
	RecordFSM       *fsm.FSM
	CurrentRecord   *Record
	CurrentSection  string
	CurrentQuestion int
	LastMessageID   int
	LastPrompt      botport.BotMessage
	ListOffset      int
	Mu              sync.Mutex
}

func NewRecord() *Record {
	return &Record{
		Data:    make(map[string]string),
		IsSaved: false,
	}
}
