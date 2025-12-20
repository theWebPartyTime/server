package conditions

import (
	"time"
)

func Timer(data any, args map[string]any) <-chan struct{} {
	channel := make(chan struct{}, 1)
	waitSeconds := time.Duration(data.(int64)) * time.Second
	go func() {
		<-time.After(waitSeconds)
		channel <- struct{}{}
	}()
	return channel
}

func Input(data any, args map[string]any) <-chan struct{} {
	channel := make(chan struct{}, 1)
	go func() {
		<-args["channel"].(chan any)
		channel <- struct{}{}
	}()
	return channel
}
