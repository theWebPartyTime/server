package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/centrifugal/centrifuge"
	"github.com/gin-gonic/gin"
)

func auth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		context := r.Context()
		credentials := &centrifuge.Credentials{UserID: ""}
		newContext := centrifuge.SetCredentials(context, credentials)
		r = r.WithContext(newContext)
		h.ServeHTTP(w, r)
	})
}

func main() {
	router := gin.Default()
	node, err := centrifuge.New(centrifuge.Config{})

	if err != nil {
		log.Fatal(err)
	}

	node.OnConnect(func(client *centrifuge.Client) {
		client.OnSubscribe(func(e centrifuge.SubscribeEvent, cb centrifuge.SubscribeCallback) {
			cb(centrifuge.SubscribeReply{
				Options: centrifuge.SubscribeOptions{
					EmitPresence: true,
				},
			}, nil)
		})

		client.OnPresence(func(event centrifuge.PresenceEvent, cb centrifuge.PresenceCallback) {
			cb(centrifuge.PresenceReply{}, nil)
		})

		encoding, _ := json.Marshal(0)

		client.OnDisconnect(func(event centrifuge.DisconnectEvent) {
			node.Publish("online-count", encoding)
		})

		node.Publish("online-count", encoding)
	})

	if err := node.Run(); err != nil {
		log.Fatal(err)
	}

	wsHandler := centrifuge.NewWebsocketHandler(
		node, centrifuge.WebsocketConfig{
			CheckOrigin: func(r *http.Request) bool {
				// originHeader := r.Header.Get("Origin")
				// if originHeader == "http://localhost:8000" {
				// 	return true }

				return true
			},
		})

	router.GET("/", func(context *gin.Context) {
		context.JSON(200, gin.H{
			"message": "Welcome to WebPartyTime!",
		})
	})

	router.GET("/w", gin.WrapH(auth(wsHandler)))
	router.Run()
}
