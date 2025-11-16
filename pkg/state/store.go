package state

import (
	"log"
	"sync"
)

type Store struct {
	users      map[int64]*UserState
	fsmCreator FSMCreator
	mu         sync.Mutex
}

func NewStore(f FSMCreator) *Store {
	return &Store{
		users:      make(map[int64]*UserState),
		fsmCreator: f,
	}
}

func (s *Store) GetOrCreateUserState(userID int64, userName string) *UserState {

	s.mu.Lock()
	defer s.mu.Unlock()

	userState, exists := s.users[userID]

	if exists {

		if userState.UserName != userName {
			log.Printf("Updating username for user %d: '%s' -> '%s'", userID, userState.UserName, userName)
			userState.UserName = userName
		}

		return userState
	}

	log.Printf("Creating new state for user %d ('%s')", userID, userName)

	mainFSM := s.fsmCreator.NewMainMenuFSM()
	recordFSM := s.fsmCreator.NewRecordFSM()
	log.Printf("Fsms created for user %d ('%s')", userID, userName)
	if mainFSM == nil || recordFSM == nil {

		log.Printf("CRITICAL: Failed to initialize FSM instances for user %d", userID)

	}

	newUserState := &UserState{
		UserID:        userID,
		UserName:      userName,
		Records:       make([]*Record, 0),
		MainMenuFSM:   mainFSM,
		RecordFSM:     recordFSM,
		CurrentRecord: nil,
	}
	log.Printf("Userstate created for user %d ('%s')", userID, userName)

	s.users[userID] = newUserState
	log.Printf("Userstate saved for user %d ('%s')", userID, userName)

	return newUserState
}
