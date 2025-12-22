package main

import (
	"encoding/json"
	"log"

	"github.com/centrifugal/centrifuge"
	"github.com/theWebPartyTime/server/internal/channels"
	"github.com/theWebPartyTime/server/internal/colors"
	"github.com/theWebPartyTime/server/internal/room"
)

type createRequest struct {
	Hash string `json:"hash"`
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
		var response []byte = nil

		switch e.Method {
		case "createRoom":
			var data createRequest
			err := json.Unmarshal(e.Data, &data)
			if err == nil && data.Hash != "" {
				rmManager().Mu.Lock()
				roomCode, err := createRoom(client.UserID(), data.Hash,
					func(roomCode string, data []byte) {
						node.Publish(channels.GetPlayPrefix()+roomCode, data)
					}, func(roomCode string, data []byte) {
						node.Publish(channels.GetSpectatePrefix()+roomCode, data)
					})
				rmManager().Mu.Unlock()
				if err == nil {
					response, _ = json.Marshal(map[string]string{"code": roomCode})
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
			roomMu.RLock()
			defer roomMu.RUnlock()

			if roomFound {
				err := room.Start(false)
				if err != nil {
					centrifugeError = &centrifuge.Error{
						Code: 500, Message: err.Error(),
					}
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
			cb(centrifuge.RPCReply{Data: response}, centrifugeError)
		} else {
			cb(centrifuge.RPCReply{Data: response}, nil)
		}
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

				if channels.IsPlay(e.Channel) {
					roomMu.Lock()
					nicknames, _ := json.Marshal(response{
						Type:    "nicknames",
						Message: map[string]any{"owner": room.GetOwner(), "all": room.GetNicknames()},
					})
					client.Send(nicknames)
					roomMu.Unlock()
				}

				newNickname, _ := json.Marshal(response{
					Type:    "new nickname",
					Message: map[string]any{client.UserID(): nickname},
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
			defer rmManager().Mu.Unlock()
			room, roomMu, roomExists := rmManager().Room(roomChannel.Code)

			if roomExists {
				if room.GetOwner() == client.UserID() {
					roomMu.Lock()
					rmManager().Close(roomChannel.Code)
					roomMu.Unlock()
					playChannel := channels.GetPlayPrefix() + roomChannel.Code
					watchChannel := channels.GetSpectatePrefix() + roomChannel.Code

					unsubscribeRequest, _ := json.Marshal(response{
						Type:    "unsubscribe",
						Message: map[string]any{},
					})

					node.Publish(playChannel, unsubscribeRequest)
					node.Publish(watchChannel, unsubscribeRequest)

					log.Printf("[%v] Room closed", colors.Left(room.GetCode()))
				} else {
					room.Left(client.UserID())
				}
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

func onMessage(client *centrifuge.Client) func(centrifuge.MessageEvent) {
	return func(e centrifuge.MessageEvent) {
		rmManager().Mu.Lock()
		defer rmManager().Mu.Unlock()

		roomCode := channels.PlayRoomCode(client.Channels())

		if roomCode != "" {
			room_, roomMu, roomExists := rmManager().Room(roomCode)

			if roomExists {
				roomMu.RLock()
				defer roomMu.RUnlock()

				var input room.Input
				err := json.Unmarshal(e.Data, &input)

				if err == nil {
					input.UserID = client.UserID()
					room_.AddInput(client.UserID(), input)
				} else {
					client.Send([]byte("Bad input request"))
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
