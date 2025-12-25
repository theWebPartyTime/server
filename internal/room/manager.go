package room

import (
	"errors"
	"log"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/theWebPartyTime/server/internal/colors"
)

type ManagerConfig struct {
	codeLength           int
	allocationRetryLimit int
}

type refs struct {
	byOwner map[string]*room
	byCode  map[string]*room
}

type Manager struct {
	config ManagerConfig
	refs   refs
	Mu     sync.RWMutex
}

func (manager *Manager) Allocate(
	owner string, config Config) (*room, error) {

	var codeBuilder strings.Builder
	for i := 0; i < manager.config.allocationRetryLimit; i++ {
		manager.generateCode(&codeBuilder)
		roomCode := codeBuilder.String()
		codeBuilder.Reset()

		_, roomExists := manager.refs.byCode[roomCode]

		if !roomExists {
			room := manager.makeRoom(config, roomCode, owner)
			log.Printf("[%v] --> %v", colors.RPC(owner), colors.RPC(roomCode))
			return room, nil
		}
	}

	return nil, errors.New("Could not allocate a new room code.")
}

func (manager *Manager) Close(roomCode string) {
	room, _, ok := manager.Room(roomCode)

	if ok {
		room.Stop()
		if manager.refs.byOwner[room.owner] == room {
			delete(manager.refs.byOwner, room.owner)
		}
		delete(manager.refs.byCode, room.code)
	}
}

func (manager *Manager) Room(roomCode string) (*room, *sync.RWMutex, bool) {
	var mu *sync.RWMutex = nil
	room, exists := manager.refs.byCode[roomCode]

	if exists {
		mu = &room.mu
	}

	return room, mu, exists
}

func (manager *Manager) ByOwner(owner string) (*room, *sync.RWMutex, bool) {
	room, ok := manager.refs.byOwner[owner]

	if ok {
		return room, &room.mu, true
	}

	return nil, nil, false
}

func (manager *Manager) makeRoom(config Config, roomCode string, owner string) *room {
	room := room{
		partyFlow:      nil,
		config:         config,
		state:          Open,
		nicknames:      make(map[string]string),
		channels:       map[string]chan any{"input-ready": make(chan any)},
		inputs:         make(map[string]Input),
		nicknameExists: make(map[string]any),
		onStart:        func() {},
		owner:          owner,
		code:           roomCode,
		createdAt:      time.Now(),
	}

	manager.refs.byCode[roomCode] = &room
	manager.refs.byOwner[owner] = &room
	return &room
}

func (manager *Manager) generateCode(builder *strings.Builder) {
	for i := 0; i < manager.config.codeLength; i++ {
		letter := rune(rand.IntN(24) + 65)
		builder.WriteRune(letter)
	}
}
