package room

func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		codeLength:           9,
		allocationRetryLimit: 3,
	}
}

func DefaultRoomConfig() Config {
	return Config{
		transferOwnership: false,
		allowSpectators:   true,
		rejectJoins:       false,
		allowAnonymous:    false,
		autoStart:         true,
	}
}

func NewManager(config ManagerConfig) *Manager {
	return &Manager{
		config: config,
		refs: refs{
			codeByOwner: make(map[string]string),
			roomByCode:  make(map[string]*room),
			ownerByCode: make(map[string]string),
		},
	}
}
