package main

import (
	"log"

	"github.com/centrifugal/centrifuge"
	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()
	router.SetTrustedProxies(nil)

	node, err := centrifuge.New(centrifugeMainConfig())

	if err != nil {
		log.Fatal(err)
	}

	node.OnConnect(func(client *centrifuge.Client) {
		client.OnPresenceStats(onPresenceStats())
		client.OnRPC(onRPC(node, client))
		client.OnSubscribe(onSubscribe(client))
		client.OnUnsubscribe(onUnsubscribe(node, client))
		client.OnMessage(onMessage(client))
	})

	if err := node.Run(); err != nil {
		log.Fatal(err)
	}

	wsHandler := centrifuge.NewWebsocketHandler(node, wsMainConfig())

	router.GET("/", root)
	router.GET(socketPath, gin.WrapH(auth(wsHandler)))

	router.Run("0.0.0.0:8080")
}
