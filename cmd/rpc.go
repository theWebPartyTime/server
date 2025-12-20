package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/theWebPartyTime/server/internal/colors"
	"github.com/theWebPartyTime/server/internal/conditions"
	"github.com/theWebPartyTime/server/internal/input"
	"github.com/theWebPartyTime/server/internal/partyflow"
	"github.com/theWebPartyTime/server/internal/room"
)

func createRoom(owner string, hash string,
	sendToPlayers func(string, []byte), sendToSpectators func(string, []byte)) (string, error) {

	filePath := scriptsPath + hash
	fileStat, err := os.Stat(filePath)

	if err != nil || fileStat.IsDir() {
		return "", fmt.Errorf("Could not get WebPartySpec at hash <%v>.", hash)
	}

	roomCode, err := rmManager().AllocateRoom(owner, room.DefaultRoomConfig())

	if err != nil {
		return "", fmt.Errorf("Room allocation failed:\n\t- %w", err)
	}

	log.Printf("[%v] --> %v", colors.RPC(owner), colors.RPC(roomCode))
	partyFlow, err := partyflow.New().FromFile(filePath, os.Stdout)

	if err != nil {
		return "", fmt.Errorf("PartyFlow build failed:\n\t- %w", err)
	}

	partyFlow.AddInputChecker("text", input.GetTextChecker())
	partyFlow.AddCondition("timer", conditions.Timer, nil)

	channel, _ := rmManager().GetChannel(roomCode, "input-ready")
	partyFlow.AddCondition("inputBased", conditions.Input,
		map[string]any{"channel": channel},
	)

	partyFlow.OnMove(func() {
		rmManager().ClearInputs(roomCode)
	})

	partyFlow.OnQuery(func(partyQuery *partyflow.PartyQuery) {
		if partyQuery.Input != nil {
			input := make(map[string]any)
			for k, v := range partyQuery.Input {
				input[k] = v
			}

			delete(input, "correct")
			input["step"] = partyQuery.Step

			inputData, _ := json.Marshal(input)
			sendToPlayers(roomCode, inputData)
		}

		if partyQuery.Layout != nil {
			layoutData, _ := json.Marshal(partyQuery.Layout)
			sendToSpectators(roomCode, layoutData)
		}
	})

	partyFlow.OnPickWinners(func(partyQuery *partyflow.PartyQuery) []string {
		winners := []string{}
		inputs := rmManager().GetInputs(roomCode)
		queryType := partyQuery.Input["type"].(string)

		if queryType == "voting" {
			voteMap := make(map[string]int)

			for _, input := range inputs {
				contentType := input.Type
				log.Printf("User passed type %v on step %v as current step was at %v and type - %v\n",
					contentType, input.Step, partyQuery.Step, queryType)

				if input.Step != partyQuery.Step || contentType != queryType {
					log.Print("User input relevance check failed")
					continue
				}

				_, ok := voteMap[input.Content]

				if !ok {
					voteMap[input.Content] = 0
				}

				voteMap[input.Content] += 1
			}

			votingWinner := ""
			maxVotes := 0

			for user, votes := range voteMap {
				if votes > maxVotes {
					votingWinner = user
					maxVotes = votes
				}
			}

			log.Printf("Voting concluded with: %v", voteMap)

			if votingWinner != "" {
				winners = append(winners, votingWinner)
			}
		} else {
			for user, input := range inputs {
				contentType := input.Type
				log.Printf("User passed type %v\n", contentType)

				if input.Step != partyQuery.Step || contentType != queryType {
					log.Print("User input relevance check failed")
					continue
				}

				correct := partyQuery.Input["correct"]
				checker := partyFlow.GetInputChecker(queryType)

				log.Printf("User sent <%v> to compare against <%v>\n", input.Content, correct)

				if correct == "pick" {
					limits := partyQuery.Input["limits"].([]any)
					correct = checker.Pick(limits)
					log.Printf("Picked correct option to be %v\n", correct)
				}

				if checker.IsCorrect(input.Content, correct) {
					log.Printf("User %v won\n", user)
					winners = append(winners, user)
				}
			}
		}

		return winners
	})

	partyFlow.SetGetVoteCandidates(func() map[string]string {
		res := make(map[string]string)

		for _, input := range rmManager().GetInputs(roomCode) {
			res[input.UserID] = input.Content
		}

		return res
	})

	partyFlow.OnFinished(func() {
		null, _ := json.Marshal(nil)
		sendToPlayers(roomCode, null)
		sendToSpectators(roomCode, null)
		rmManager().StopRoom(roomCode)
	})

	rmManager().AttachPartyFlow(roomCode, partyFlow)

	return roomCode, nil
}

func startRoom(roomCode string) error {
	return rmManager().StartRoom(roomCode, false)
}
