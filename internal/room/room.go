package room

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/theWebPartyTime/server/internal/colors"
	"github.com/theWebPartyTime/server/internal/partyflow"
)

type Config struct {
	AllowSpectators bool
	RejectJoins     bool
	AllowAnonymous  bool
	AutoStart       bool
}

type Input struct {
	Type    string         `json:"type"`
	Content map[string]any `json:"content"`
}

type TypeInput struct {
	Step    int    `json:"step"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

type roomState int

const (
	Open roomState = iota
	Ongoing
)

type room struct {
	config Config
	state  roomState
	mu     sync.RWMutex

	code           string
	owner          string
	channels       map[string]chan any
	inputs         map[string]Input
	nicknames      map[string]string
	nicknameExists map[string]any
	createdAt      time.Time
	partyFlow      *partyflow.PartyFlow
	onStart        func()
}

func (room *room) CanJoin(user string, spectatorMode bool) bool {
	if room.isOwner(user) {
		return true
	}

	return !((room.config.RejectJoins) ||
		(room.config.AllowSpectators && room.state == Ongoing && !spectatorMode) ||
		(!room.config.AllowSpectators && spectatorMode))
}

func (room *room) Joined(user string, nickname string) string {
	_, exists := room.nicknameExists[nickname]

	if exists {
		nickname = nickname + " (" + user[:2] + "...)"
	}

	room.nicknameExists[nickname] = nil
	room.nicknames[user] = nickname
	return nickname
}

func (room *room) SetConfig(newConfig Config) {
	room.config = newConfig
}

func (room *room) SetOnStart(onStart func()) {
	room.onStart = onStart
}

func (room *room) Left(user string) {
	nickname, _ := room.nicknames[user]
	delete(room.nicknameExists, nickname)
	delete(room.nicknames, user)
	room.removeInput(user)
	log.Printf("[%v] left %v", colors.Left(user), colors.Left(room.GetCode()))
}

func (room *room) GetNicknames() map[string]string {
	return room.nicknames
}

func (room *room) GetCreatedAt() time.Time {
	return room.createdAt
}

func (room *room) AttachPartyFlow(partyFlow *partyflow.PartyFlow) {
	room.partyFlow = partyFlow
}

func (room *room) AddChannel(name string, channel chan any) {
	room.channels[name] = channel
}

func (room *room) RemoveChannel(name string, channel chan any) {
	delete(room.channels, name)
}

func (room *room) GetChannel(roomCode string, name string) (chan any, bool) {
	channel, ok := room.channels[name]
	return channel, ok
}

func (room *room) GetInputReadyChannel() chan any {
	return room.channels["input-ready"]
}

func (room *room) AddInput(user string, input Input) {
	_, ok := room.inputs[user]
	if !ok {
		room.inputs[user] = input
	} else if room.state == Open {
		room.removeInput(user)
	}

	room.checkInputsReady()
}

func (room *room) GetInputs() map[string]Input {
	return room.inputs
}

func (room *room) ClearInputs() {
clear:
	for {
		select {
		case <-room.channels["input-ready"]:
		default:
			break clear
		}
	}

	clear(room.inputs)
}

func (room *room) removeInput(user string) {
	delete(room.inputs, user)
	room.checkInputsReady()
}

func (room *room) checkInputsReady() {
	inputs := room.inputs
	online := len(room.nicknames) - 1

	if room.state == Open && room.config.AutoStart && online != 0 && len(inputs) == online {
		// room.Start(false)
		log.Printf("\nstarting 123123\n")
		room.onStart()
		room.ClearInputs()
	} else if room.state == Ongoing {
		filteredByStep := 0

		for _, input := range inputs {
			if input.Content["step"].(float64) == float64(room.partyFlow.GetStep()) {
				filteredByStep += 1
			}
		}

		if filteredByStep == online {
			channel, _ := room.channels["input-ready"]
			channel <- struct{}{}
		}
	}
}

func (room *room) GetCode() string {
	return room.code
}

func (room *room) GetOwner() string {
	return room.owner
}

func (room *room) Start(restartIfOngoing bool) error {
	if restartIfOngoing && room.state == Ongoing {
		room.Stop()
	}

	if room.state == Open {
		room.state = Ongoing
		go room.partyFlow.Start()

		log.Printf("--> %v started", colors.RPC(room.code))

		return nil
	}

	return errors.New("Room currently has an ongoing game.")
}

func (room *room) Stop() {
	room.partyFlow.Stop()

	for _, channel := range room.channels {
	clear:
		for {
			select {
			case <-channel:
			default:
				break clear
			}
		}
	}

	room.state = Open
}

func (room *room) isOwner(owner string) bool {
	return room.owner == owner
}
