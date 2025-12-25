package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/centrifugal/centrifuge"
	"github.com/theWebPartyTime/server/internal/channels"
	"github.com/theWebPartyTime/server/internal/colors"
	"github.com/theWebPartyTime/server/internal/room"
)

type createRequest struct {
	Hash string `json:"hash"`
}

type request struct {
	Type    string         `json:"type"`
	Content map[string]any `json:"content"`
}

type response struct {
	Type    string         `json:"type"`
	Message map[string]any `json:"message"`
}

func onRPC(node *centrifuge.Node, client *centrifuge.Client) func(centrifuge.RPCEvent, centrifuge.RPCCallback) {
	return func(e centrifuge.RPCEvent, cb centrifuge.RPCCallback) {
		log.Printf("[%v] RPC.%v()",
			colors.RPC(client.UserID()), colors.RPC(e.Method))

		var centrifugeError *centrifuge.Error = nil
		var RPCResponse []byte = nil

		switch e.Method {
		case "createRoom":
			var data createRequest
			err := json.Unmarshal(e.Data, &data)
			if err == nil && data.Hash != "" {
				rmManager().Mu.Lock()
				roomCode, startedAt, err := createRoom(client.UserID(), data.Hash,
					func(roomCode string, data []byte) {
						node.Publish(channels.GetPlayPrefix()+roomCode, data)
					}, func(roomCode string, data []byte) {
						node.Publish(channels.GetSpectatePrefix()+roomCode, data)
					})

				room, roomMu, _ := rmManager().Room(roomCode)
				roomMu.Lock()
				room.SetOnStart(func() {
					log.Printf("\nstarted 123\n")
					startMsg, _ := json.Marshal(response{
						Type:    "room_started",
						Message: map[string]any{},
					})
					node.Publish(channels.GetPlayPrefix()+room.GetCode(), startMsg)
					node.Publish(channels.GetSpectatePrefix()+room.GetCode(), startMsg)
				})
				roomMu.Unlock()
				rmManager().Mu.Unlock()
				if err == nil {
					RPCResponse, _ = json.Marshal(map[string]string{
						"code": roomCode, "startedAt": startedAt.Format(time.RFC3339)})
				} else {
					centrifugeError = &centrifuge.Error{Code: 500, Message: err.Error()}
				}
			} else {
				centrifugeError = &centrifuge.Error{
					Code: 400, Message: "Data provided to the remote procedure is invalid."}
			}

		case "startRoom":
			rmManager().Mu.Lock()
			defer rmManager().Mu.Unlock()

			room, roomMu, roomFound := rmManager().ByOwner(client.UserID())

			if roomFound {
				roomMu.RLock()
				defer roomMu.RUnlock()
				err := room.Start(false)
				if err != nil {
					centrifugeError = &centrifuge.Error{
						Code: 500, Message: err.Error(),
					}
				} else {
					startMsg, _ := json.Marshal(response{
						Type:    "room_started",
						Message: map[string]any{},
					})
					node.Publish(channels.GetPlayPrefix()+room.GetCode(), startMsg)
					node.Publish(channels.GetSpectatePrefix()+room.GetCode(), startMsg)
				}
			} else {
				centrifugeError = &centrifuge.Error{
					Code: 400, Message: "User does not own any room.",
				}
			}

		default:
			centrifugeError = centrifuge.ErrorMethodNotFound
		}

		if centrifugeError != nil {
			cb(centrifuge.RPCReply{Data: RPCResponse}, centrifugeError)
		} else {
			cb(centrifuge.RPCReply{Data: RPCResponse}, nil)
		}
	}
}

func onDisconnect(client *centrifuge.Client) func(centrifuge.DisconnectEvent) {
	return func(e centrifuge.DisconnectEvent) {
	}
}

func onSubscribe(node *centrifuge.Node, client *centrifuge.Client) func(centrifuge.SubscribeEvent, centrifuge.SubscribeCallback) {
	return func(e centrifuge.SubscribeEvent, cb centrifuge.SubscribeCallback) {
		rmManager().Mu.Lock()
		defer rmManager().Mu.Unlock()

		if !channels.IsValid(e.Channel) {
			cb(centrifuge.SubscribeReply{}, centrifuge.ErrorUnknownChannel)
			return
		}

		if !channels.IsMain(e.Channel) {
			roomChannel := channels.AsRoomChannel(e.Channel)
			if roomChannel != nil {
				room, roomMu, roomExists := rmManager().Room(roomChannel.Code)

				if !roomExists {
					cb(centrifuge.SubscribeReply{}, centrifuge.ErrorUnknownChannel)
					return
				}

				var nickname string
				json.Unmarshal(e.Data, &nickname)

				if !room.CanJoin(client.UserID(), channels.IsWatch(e.Channel)) {
					cb(centrifuge.SubscribeReply{}, centrifuge.ErrorPermissionDenied)
					return
				}

				nickname = room.Joined(client.UserID(), nickname)

				roomMu.Lock()
				if channels.IsPlay(e.Channel) {
					allNicknames := room.GetNicknames()
					playerNicknames := make(map[string]string)
					ownerNickname := ""

					for user, nickname := range allNicknames {
						if user != room.GetOwner() {
							playerNicknames[user] = nickname
						} else {
							ownerNickname = nickname
						}
					}

					nicknames, _ := json.Marshal(response{
						Type: "nicknames",
						Message: map[string]any{
							"owner": ownerNickname, "all": playerNicknames, "self": client.UserID()},
					})

					client.Send(nicknames)
				}

				roomCreatedAt, _ := json.Marshal(response{
					Type:    "room_created_at",
					Message: map[string]any{"timestamp": room.GetCreatedAt().Format(time.RFC3339)},
				})

				roomMu.Unlock()
				client.Send(roomCreatedAt)

				newNickname, _ := json.Marshal(response{
					Type:    "new_nickname",
					Message: map[string]any{"id": client.UserID(), "nickname": nickname},
				})

				node.Publish(e.Channel, newNickname)
				node.Publish(channels.GetSpectatePrefix()+room.GetCode(), newNickname)

				channels.UnsubscribeFromRooms(client.Channels(),
					func(channel string) {
						client.Unsubscribe(channel)
					},
				)
			}
		}

		log.Printf("[%v] joined %v", colors.Joined(client.UserID()), colors.Joined(e.Channel))

		cb(centrifuge.SubscribeReply{
			Options: centrifugeVisibleSubscription(),
		}, nil)
	}
}

func onUnsubscribe(node *centrifuge.Node, client *centrifuge.Client) func(e centrifuge.UnsubscribeEvent) {
	return func(e centrifuge.UnsubscribeEvent) {
		roomChannel := channels.AsRoomChannel(e.Channel)

		if roomChannel != nil {
			rmManager().Mu.Lock()

			room, roomMu, roomExists := rmManager().Room(roomChannel.Code)

			if roomExists {
				roomMu.Lock()
				if room.GetOwner() == client.UserID() {
					rmManager().Close(roomChannel.Code)
					log.Printf("[%v] Room closed", colors.Left(room.GetCode()))
					roomMu.Unlock()
					rmManager().Mu.Unlock()

					playChannel := channels.GetPlayPrefix() + roomChannel.Code
					watchChannel := channels.GetSpectatePrefix() + roomChannel.Code

					unsubscribeRequest, _ := json.Marshal(response{
						Type:    "unsubscribe",
						Message: map[string]any{},
					})

					node.Publish(playChannel, unsubscribeRequest)
					node.Publish(watchChannel, unsubscribeRequest)

				} else {
					room.Left(client.UserID())

					playChannel := channels.GetPlayPrefix() + room.GetCode()
					watchChannel := channels.GetSpectatePrefix() + room.GetCode()

					unsubscribeRequest, _ := json.Marshal(response{
						Type:    "remove_nickname",
						Message: map[string]any{"id": client.UserID()},
					})

					node.Publish(playChannel, unsubscribeRequest)
					node.Publish(watchChannel, unsubscribeRequest)

					roomMu.Unlock()
					rmManager().Mu.Unlock()
				}
			} else {
				rmManager().Mu.Unlock()
			}
		}
	}
}

func onPresenceStats() func(centrifuge.PresenceStatsEvent, centrifuge.PresenceStatsCallback) {
	return func(e centrifuge.PresenceStatsEvent, cb centrifuge.PresenceStatsCallback) {
		if channels.IsMain(e.Channel) {
			cb(centrifuge.PresenceStatsReply{}, nil)
		} else {
			cb(centrifuge.PresenceStatsReply{}, centrifuge.ErrorPermissionDenied)
		}
	}
}

func onMessage(node *centrifuge.Node, client *centrifuge.Client) func(centrifuge.MessageEvent) {
	return func(e centrifuge.MessageEvent) {
		var request request
		err := json.Unmarshal(e.Data, &request)

		if err != nil {
			client.Send([]byte("Bad input request"))
			return
		}

		rmManager().Mu.Lock()
		defer rmManager().Mu.Unlock()

		roomCode := channels.RoomCode(client.Channels())

		if roomCode != "" {
			room_, roomMu, roomExists := rmManager().Room(roomCode)

			if roomExists {
				roomMu.RLock()
				defer roomMu.RUnlock()

				switch request.Type {
				case "room_config_changed":
					if room_.GetOwner() == client.UserID() {
						options := request.Content["config"].(map[string]any)

						room_.SetConfig(room.Config{
							AllowSpectators: options["allowSpectators"].(bool),
							AllowAnonymous:  options["allowAnonymous"].(bool),
							AutoStart:       options["autoStart"].(bool),
							RejectJoins:     !options["allowJoins"].(bool),
						})
					}
				case "kick":
					playChannel := channels.GetPlayPrefix() + room_.GetCode()
					watchChannel := channels.GetSpectatePrefix() + room_.GetCode()

					unsubscribeRequest, _ := json.Marshal(response{
						Type:    "remove_nickname",
						Message: map[string]any{"id": request.Content["userID"]},
					})

					node.Publish(playChannel, unsubscribeRequest)
					node.Publish(watchChannel, unsubscribeRequest)
					node.Unsubscribe(request.Content["userID"].(string), playChannel)
				default:
					room_.AddInput(client.UserID(), room.Input{
						Type:    request.Type,
						Content: request.Content,
					})
				}
			}
		} else {
			room, roomMu, roomExists := rmManager().ByOwner(client.UserID())

			if roomExists {
				roomMu.RLock()
				room.Stop()
				roomMu.RUnlock()
			}
		}
	}
}
