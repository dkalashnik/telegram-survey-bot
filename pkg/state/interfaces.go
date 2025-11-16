package state

import "github.com/looplab/fsm"

type FSMCreator interface {
	NewMainMenuFSM() *fsm.FSM
	NewRecordFSM() *fsm.FSM
}
