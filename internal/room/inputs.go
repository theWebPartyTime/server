package room

func (manager *Manager) AddInput(roomCode string, user string, input Input) {
	room := manager.refs.roomByCode[roomCode]

	_, ok := room.inputs[user]
	if !ok {
		room.inputs[user] = input
	}

	manager.CheckInputsReady(roomCode)
}

func (manager *Manager) ClearInputs(roomCode string) {
	for len(manager.refs.roomByCode[roomCode].channels["input-ready"]) > 0 {
		<-manager.refs.roomByCode[roomCode].channels["input-ready"]
	}

	clear(manager.refs.roomByCode[roomCode].inputs)
}

func (manager *Manager) GetInputs(roomCode string) map[string]Input {
	return manager.refs.roomByCode[roomCode].inputs
}

func (manager *Manager) removeInput(roomCode string, user string) {
	delete(manager.refs.roomByCode[roomCode].inputs, user)
	manager.CheckInputsReady(roomCode)
}

func (manager *Manager) CheckInputsReady(roomCode string) {
	room := manager.GetRoom(roomCode)
	inputs := room.inputs
	online := len(room.nicknames)

	if room.state == Open && room.config.autoStart && len(inputs) == online {
		manager.StartRoom(roomCode, false)
		manager.ClearInputs(roomCode)
	} else {
		filteredByStep := 0

		for _, input := range inputs {
			if input.Step == room.partyFlow.GetStep() {
				filteredByStep += 1
			}
		}

		if filteredByStep == online {
			channel, _ := room.channels["input-ready"]
			channel <- struct{}{}
		}
	}

}
