package room

func (manager *Manager) AddChannel(roomCode string, name string, channel chan any) {
	manager.refs.roomByCode[roomCode].channels[name] = channel
}

func (manager *Manager) RemoveChannel(roomCode string, name string, channel chan any) {
	delete(manager.refs.roomByCode[roomCode].channels, name)
}

func (manager *Manager) GetChannel(roomCode string, name string) (chan any, bool) {
	channel, ok := manager.refs.roomByCode[roomCode].channels[name]
	return channel, ok
}
