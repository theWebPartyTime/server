package channels

import (
	"strings"
)

const main = "main"

const attributeSeparator = "@"
const spectateTag = "watch"
const playTag = "play"

type RoomChannel struct {
	Mode string
	Code string
}

func RoomCode(channels []string) string {
	for i := len(channels) - 1; i >= 0; i-- {
		if IsPlay(channels[i]) || IsWatch(channels[i]) {
			roomChannel := AsRoomChannel(channels[i])
			return roomChannel.Code
		}
	}

	return ""
}

func AsRoomChannel(channel string) *RoomChannel {
	channelAttributes := strings.SplitN(channel, attributeSeparator, 2)

	if len(channelAttributes) == 1 {
		return nil
	}

	return &RoomChannel{
		Code: channelAttributes[1],
		Mode: channelAttributes[0],
	}
}

func GetPlayPrefix() string {
	return playTag + attributeSeparator
}

func GetSpectatePrefix() string {
	return spectateTag + attributeSeparator
}

func IsRoom(channel string) bool {
	return strings.ContainsAny(channel, attributeSeparator)
}

func IsPlay(channel string) bool {
	return strings.Contains(channel, playTag+attributeSeparator)
}

func IsWatch(channel string) bool {
	return strings.Contains(channel, spectateTag+attributeSeparator)
}

func IsMain(channel string) bool {
	return channel == main
}

func IsValid(channel string) bool {
	if channel == main {
		return true
	}

	roomChannel := AsRoomChannel(channel)
	if roomChannel != nil {
		return (roomChannel.Mode == spectateTag || roomChannel.Mode == playTag)
	}

	return false
}

func UnsubscribeFromRooms(channels []string, unsubscribe func(channel string)) {
	for _, channel := range channels {
		if IsRoom(channel) {
			unsubscribe(channel)
		}
	}
}
