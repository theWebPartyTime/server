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
	Type    string            `json:"type"`
	Message map[string]string `json:"message"`
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

			roomCode, roomFound := rmManager().RoomCodeByOwner(client.UserID())
			rmManager().GetRoomMu(roomCode).RLock()
			defer rmManager().GetRoomMu(roomCode).RUnlock()

			if roomFound {
				err := startRoom(roomCode)
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

func onUnsubscribe(node *centrifuge.Node, client *centrifuge.Client) func(e centrifuge.UnsubscribeEvent) {
	return func(e centrifuge.UnsubscribeEvent) {
		if !channels.IsMain(e.Channel) {
			roomChannel := channels.AsRoomChannel(e.Channel)

			rmManager().Mu.Lock()
			defer rmManager().Mu.Unlock()

			rmManager().GetRoomMu(roomChannel.Code).Lock()
			defer rmManager().GetRoomMu(roomChannel.Code).Unlock()

			rmManager().Left(func() []string {
				playPresence, _ := node.Presence(channels.GetPlayPrefix() + roomChannel.Code)
				watchPresence, _ := node.Presence(channels.GetSpectatePrefix() + roomChannel.Code)
				clients := make([]string, 0)

				for _, clientInfo := range playPresence.Presence {
					clients = append(clients, clientInfo.UserID)
				}

				for _, clientInfo := range watchPresence.Presence {
					clients = append(clients, clientInfo.UserID)
				}

				return clients
			}, roomChannel.Code, client.UserID())
		}

		log.Printf("[%v] left %v", colors.Left(client.UserID()), colors.Left(e.Channel))
	}
}

func onSubscribe(client *centrifuge.Client) func(centrifuge.SubscribeEvent, centrifuge.SubscribeCallback) {
	return func(e centrifuge.SubscribeEvent, cb centrifuge.SubscribeCallback) {
		roomExists := false
		roomChannel := channels.AsRoomChannel(e.Channel)

		var nickname string
		json.Unmarshal(e.Data, &nickname)

		rmManager().Mu.Lock()
		defer rmManager().Mu.Unlock()

		if roomChannel != nil && rmManager().RoomExists(roomChannel.Code) {
			roomExists = true
		}

		if !channels.IsValid(e.Channel, roomExists) {
			cb(centrifuge.SubscribeReply{}, centrifuge.ErrorUnknownChannel)
			return
		}

		if !channels.IsMain(e.Channel) {
			if !rmManager().CanJoin(client.UserID(), roomChannel.Code, channels.IsWatch(e.Channel)) {
				cb(centrifuge.SubscribeReply{}, centrifuge.ErrorPermissionDenied)
				return
			}

			if channels.IsPlay(e.Channel) {
				rmManager().GetRoomMu(roomChannel.Code).Lock()
				nicknames, _ := json.Marshal(response{
					Type:    "nicknames",
					Message: rmManager().GetNicknames(roomChannel.Code),
				})
				client.Send(nicknames)
				rmManager().Joined(roomChannel.Code, client.UserID(), nickname)
				rmManager().GetRoomMu(roomChannel.Code).Unlock()
			}

			channels.UnsubscribeFromRooms(client.Channels(),
				func(channel string) {
					client.Unsubscribe(channel)
				},
			)
		}

		log.Printf("[%v] joined %v", colors.Joined(client.UserID()), colors.Joined(e.Channel))

		cb(centrifuge.SubscribeReply{
			Options: centrifugeVisibleSubscription(),
		}, nil)
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

		roomCode := channels.PlayingRoomCode(client.Channels())

		if roomCode != "" {
			rmManager().GetRoomMu(roomCode).RLock()
			defer rmManager().GetRoomMu(roomCode).RUnlock()
			var input room.Input
			err := json.Unmarshal(e.Data, &input)

			if err == nil {
				input.UserID = client.UserID()
				rmManager().AddInput(roomCode, client.UserID(), input)
			} else {
				client.Send([]byte("Bad input request"))
			}
		} else {
			roomCode, exists := rmManager().RoomCodeByOwner(client.UserID())

			if exists {
				rmManager().GetRoomMu(roomCode).RLock()
				rmManager().StopRoom(roomCode)
				rmManager().GetRoomMu(roomCode).RUnlock()
			}
		}
	}
}
