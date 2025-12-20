package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/centrifugal/centrifuge"
	"github.com/theWebPartyTime/server/internal/room"
)

const socketPath = "/join"
const scriptsPath = "../examples/"

var roomManagerOnce sync.Once
var roomManager *room.Manager

func rmManager() *room.Manager {
	roomManagerOnce.Do(func() {
		roomManager = room.NewManager(room.DefaultManagerConfig())
	})

	return roomManager
}

func checkOrigin(r *http.Request) bool {
	// originHeader := r.Header.Get("Origin")
	// if originHeader == "http://localhost:8000" {
	// 	return true }

	return true
}

func wsMainConfig() centrifuge.WebsocketConfig {
	return centrifuge.WebsocketConfig{
		CheckOrigin: checkOrigin,
	}
}

func centrifugeMainConfig() centrifuge.Config {
	return centrifuge.Config{
		LogLevel: centrifuge.LogLevelError,
		LogHandler: func(e centrifuge.LogEntry) {
			log.Printf("%v %v", e.Message, e.Fields)
		},
	}
}

func centrifugeVisibleSubscription() centrifuge.SubscribeOptions {
	return centrifuge.SubscribeOptions{
		EmitPresence:  true,
		EmitJoinLeave: true,
		PushJoinLeave: true,
	}
}
