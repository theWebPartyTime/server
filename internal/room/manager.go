package room

import (
	"errors"
	"log"
	"math/rand/v2"
	"strings"
	"sync"

	"github.com/theWebPartyTime/server/internal/colors"
)

type ManagerConfig struct {
	codeLength           int
	allocationRetryLimit int
}

type refs struct {
	codeByOwner map[string]string
	roomByCode  map[string]*room
	ownerByCode map[string]string
}

type Manager struct {
	config ManagerConfig
	refs   refs
	Mu     sync.RWMutex
}

func (manager *Manager) makeRoom(config Config, roomCode string, owner string) *room {
	room := room{
		partyFlow:   nil,
		config:      config,
		state:       Open,
		nicknames:   make(map[string]string),
		channels:    make(map[string]chan any),
		inputs:      make(map[string]Input),
		hasNickname: make(map[string]any),
	}

	manager.refs.roomByCode[roomCode] = &room
	manager.refs.codeByOwner[owner] = roomCode
	manager.refs.ownerByCode[roomCode] = owner

	return &room
}

func (manager *Manager) AllocateRoom(
	owner string, config Config) (string, error) {

	var codeBuilder strings.Builder
	for i := 0; i < manager.config.allocationRetryLimit; i++ {
		manager.generateCode(&codeBuilder)
		roomCode := codeBuilder.String()
		codeBuilder.Reset()

		_, roomExists := manager.refs.roomByCode[roomCode]

		if !roomExists {
			manager.makeRoom(config, roomCode, owner)
			manager.AddChannel(roomCode, "input-ready", make(chan any))
			return roomCode, nil
		}
	}

	return "", errors.New("Could not allocate a new room code.")
}

func (manager *Manager) closeRoom(roomCode string) {
	manager.StopRoom(roomCode)
	// owner, _ := manager.refs.ownerByCode[roomCode]

	// delete(manager.refs.codeByOwner, owner)
	// delete(manager.refs.ownerByCode, roomCode)
	// delete(manager.refs.roomByCode, roomCode)
}

func (manager *Manager) ownsRoom(owner string, roomCode string) bool {
	ownsRoomCode, ownsAny := manager.refs.codeByOwner[owner]

	if ownsAny {
		return ownsRoomCode == roomCode
	}

	return false
}

func (manager *Manager) generateCode(builder *strings.Builder) {
	for i := 0; i < manager.config.codeLength; i++ {
		letter := rune(rand.IntN(24) + 65)
		builder.WriteRune(letter)
	}
}

func (manager *Manager) GetRoom(roomCode string) *room {
	return manager.refs.roomByCode[roomCode]
}

func (manager *Manager) StartRoom(roomCode string, restartIfOngoing bool) error {
	room := manager.refs.roomByCode[roomCode]

	if restartIfOngoing && room.state == Ongoing {
		manager.StopRoom(roomCode)
	}

	if room.state == Open {
		go manager.refs.roomByCode[roomCode].partyFlow.Start()
		room.state = Ongoing

		log.Printf("--> %v started", colors.RPC(roomCode))
		return nil
	}

	return errors.New("Room currently has an ongoing game.")
}

func (manager *Manager) StopRoom(roomCode string) {
	room, _ := manager.refs.roomByCode[roomCode]
	room.partyFlow.Stop()
	room.state = Open
}

func (manager *Manager) RoomExists(roomCode string) bool {
	_, ok := manager.refs.roomByCode[roomCode]
	return ok
}

func (manager *Manager) RoomCodeByOwner(owner string) (string, bool) {
	roomCode, ok := manager.refs.codeByOwner[owner]
	return roomCode, ok
}

func (manager *Manager) CanJoin(user string, roomCode string, spectatorMode bool) bool {
	room := manager.refs.roomByCode[roomCode]
	if !manager.ownsRoom(user, roomCode) {
		if (room.config.rejectJoins) ||
			(room.state == Ongoing && !spectatorMode) ||
			(!room.config.allowSpectators && spectatorMode) {
			return false
		}
	}

	return true
}

func (manager *Manager) GetRoomMu(roomCode string) *sync.RWMutex {
	return &manager.refs.roomByCode[roomCode].mu
}
