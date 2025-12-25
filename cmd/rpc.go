package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/theWebPartyTime/server/internal/conditions"
	"github.com/theWebPartyTime/server/internal/input"
	"github.com/theWebPartyTime/server/internal/partyflow"
	"github.com/theWebPartyTime/server/internal/room"
)

func createRoom(owner string, hash string,
	sendToPlayers func(string, []byte), sendToSpectators func(string, []byte)) (string, time.Time, error) {

	filePath := scriptsPath + hash
	partyFlow, err := partyflow.New().FromFile(filePath, os.Stdout)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("PartyFlow build failed:\n\t- %w", err)
	}

	room, err := rmManager().Allocate(owner, room.DefaultRoomConfig())
	if err != nil {
		return "", time.Time{}, fmt.Errorf("Room allocation failed:\n\t- %w", err)
	}

	partyFlow.AddInputChecker("text", input.GetTextChecker())
	partyFlow.AddCondition("timer", conditions.Timer, nil)
	partyFlow.AddCondition("inputBased", conditions.Input,
		map[string]any{"channel": room.GetInputReadyChannel()},
	)

	partyFlow.OnQuery(func(partyQuery *partyflow.PartyQuery) {
		if partyQuery.Input != nil {
			input := make(map[string]any)
			for k, v := range partyQuery.Input {
				input[k] = v
			}

			delete(input, "correct")
			input["step"] = partyQuery.Step

			inputData, _ := json.Marshal(input)
			sendToPlayers(room.GetCode(), inputData)
		}

		if partyQuery.Layout != nil {
			layoutData, _ := json.Marshal(partyQuery.Layout)
			sendToSpectators(room.GetCode(), layoutData)
		}
	})

	partyFlow.OnGetWinners(func(partyQuery *partyflow.PartyQuery) []string {
		winners := []string{}
		if partyQuery.Input == nil {
			return winners
		}

		inputs := room.GetInputs()
		var voteMap map[string]int = nil

		queryType := partyQuery.Input["type"].(string)
		if queryType == "vote all" {
			voteMap = make(map[string]int)
		}

		for userID, input := range inputs {
			inputType := input.Type
			if inputType != "input" {
				log.Printf("Wrong input type sent in by <%s>", userID)
				continue
			}

			step, ok := input.Content["step"].(float64)
			if !ok {
				log.Printf("Step not specified or specified incorrectly by <%s>", userID)
				continue
			}

			message, ok := input.Content["message"].(string)
			if !ok {
				log.Printf("Content input not specified or specified incorrectly by <%s>", userID)
				continue
			}

			contentType, ok := input.Content["type"].(string)
			log.Printf("User passed type %v\n", contentType)
			if !ok {
				log.Printf("Content input type not specified or specified incorrectly by <%s>", userID)
				continue
			}

			if contentType != queryType || step != float64(partyQuery.Step) {
				log.Printf("User <%s> input relevance check failed: tried step %v, type %v (when need step %v, type %v)",
					userID, step, contentType, partyQuery.Step, queryType)
				continue
			}

			if queryType == "vote all" {
				_, ok = voteMap[message]
				if !ok {
					voteMap[message] = 0
				}
				voteMap[message] += 1

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
				correct := partyQuery.Input["correct"]
				checker := partyFlow.GetInputChecker(queryType)

				log.Printf("User sent <%v> to compare against <%v>\n", input.Content, correct)

				if correct == "pick" {
					limits := partyQuery.Input["limits"].([]any)
					correct = checker.Pick(limits)
					log.Printf("Picked correct option to be %v\n", correct)
				}

				if checker.IsCorrect(message, correct) {
					log.Printf("User %v won\n", userID)
					winners = append(winners, userID)
				}
			}
		}

		return winners
	})

	partyFlow.OnGetInputs(func(partyQuery *partyflow.PartyQuery) map[string]string {
		res := make(map[string]string)

		for userID, input := range room.GetInputs() {
			res[userID] = input.Content["message"].(string)
		}

		return res
	})

	partyFlow.OnMove(func() {
		room.ClearInputs()
	})

	partyFlow.OnFinished(func() {
		endMsg, _ := json.Marshal(response{
			Type:    "room_ended",
			Message: map[string]any{},
		})

		sendToPlayers(room.GetCode(), endMsg)
		sendToSpectators(room.GetCode(), endMsg)
		room.Stop()
	})

	room.AttachPartyFlow(partyFlow)

	return room.GetCode(), room.GetCreatedAt(), nil
}
