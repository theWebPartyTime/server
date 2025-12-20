package room

import (
	"sync"

	"github.com/theWebPartyTime/server/internal/partyflow"
)

type Config struct {
	transferOwnership bool
	allowSpectators   bool
	rejectJoins       bool
	allowAnonymous    bool
	autoStart         bool
}

type roomState int

const (
	Open roomState = iota
	Ongoing
)

type Input struct {
	Step    int    `json:"step"`
	Content string `json:"content"`
	UserID  string `json:"userID"`
	Type    string `json:"type"`
}

type room struct {
	partyFlow   *partyflow.PartyFlow
	config      Config
	state       roomState
	channels    map[string]chan any
	inputs      map[string]Input
	nicknames   map[string]string
	hasNickname map[string]any
	mu          sync.RWMutex
}

func (manager *Manager) Left(getRoomClients func() []string, roomCode string, user string) {
	nickname, _ := manager.refs.roomByCode[roomCode].nicknames[user]
	delete(manager.refs.roomByCode[roomCode].nicknames, user)
	delete(manager.refs.roomByCode[roomCode].hasNickname, nickname)
}

func (manager *Manager) GetNicknames(roomCode string) map[string]string {
	return manager.refs.roomByCode[roomCode].nicknames
}

func (manager *Manager) Joined(roomCode string, user string, nickname string) {
	_, exists := manager.refs.roomByCode[roomCode].hasNickname[nickname]

	if exists {
		nickname = nickname + " (" + user[:2] + "...)"
	}

	manager.refs.roomByCode[roomCode].hasNickname[nickname] = nil
	manager.refs.roomByCode[roomCode].nicknames[user] = nickname
}

func (manager *Manager) AttachPartyFlow(roomCode string, partyFlow *partyflow.PartyFlow) {
	manager.refs.roomByCode[roomCode].partyFlow = partyFlow
}
