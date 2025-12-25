package room

func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		codeLength:           9,
		allocationRetryLimit: 3,
	}
}

func DefaultRoomConfig() Config {
	return Config{
		RejectJoins:     true,
		AutoStart:       false,
		AllowSpectators: false,
		AllowAnonymous:  false,
	}
}

func NewManager(config ManagerConfig) *Manager {
	return &Manager{
		config: config,
		refs: refs{
			byOwner: make(map[string]*room),
			byCode:  make(map[string]*room),
		},
	}
}
