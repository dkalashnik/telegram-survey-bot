package fsm

import (
	"github.com/dkalashnik/telegram-survey-bot/pkg/state"

	"github.com/looplab/fsm"
)

type fsmCreatorImpl struct{}

func (fc *fsmCreatorImpl) NewMainMenuFSM() *fsm.FSM {
	return NewMainMenuFSM(StateIdle)
}

func (fc *fsmCreatorImpl) NewRecordFSM() *fsm.FSM {
	return NewRecordFSM(StateRecordIdle)
}

func NewFSMCreator() state.FSMCreator {
	return &fsmCreatorImpl{}
}
