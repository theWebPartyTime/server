package partyflow

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/theWebPartyTime/server/internal/colors"
	"github.com/theWebPartyTime/server/internal/input"
)

type Standing struct {
	LastInput string `json:"lastInput"`
	WinCount  int    `json:"winCount"`
}

type PartyFlow struct {
	logger *log.Logger

	start   *PartyQuery
	current *PartyQuery

	inputCheckers     map[string]input.Checker
	conditionCheckers map[string]func(any, map[string]any) <-chan struct{}
	conditionArgs     map[string]map[string]any

	onQuery      func(*PartyQuery)
	onMove       func()
	onFinished   func()
	onGetInputs  func(*PartyQuery) map[string]string
	onGetWinners func(*PartyQuery) []string
	standings    map[string]Standing

	stepCounter    atomic.Int64
	skipGetWinners bool

	context context.Context
	stop    context.CancelFunc
}

type conditionalMove struct {
	to   *PartyQuery
	when map[string]any
}

type PartyQuery struct {
	Name         string
	Overviewer   map[string]any
	Vote         map[string]any
	Input        map[string]any
	Layout       map[string]any
	NextVariants []conditionalMove
	Step         int
}

func New() *PartyFlow {
	partyFlow := PartyFlow{
		start:             nil,
		conditionCheckers: make(map[string]func(any, map[string]any) <-chan struct{}),
		inputCheckers:     make(map[string]input.Checker),
		conditionArgs:     make(map[string]map[string]any),
		standings:         make(map[string]Standing),
		onQuery:           func(pq *PartyQuery) {},
		onGetWinners:      func(pq *PartyQuery) []string { return []string{} },
		skipGetWinners:    false,
		context:           nil,
		stop:              nil,
		onMove:            func() {},
		onFinished:        func() {},
		onGetInputs:       func(*PartyQuery) map[string]string { return map[string]string{} },
		logger:            nil,
	}

	return &partyFlow
}

func (partyFlow *PartyFlow) next(choice int) (*PartyQuery, error) {
	paths := len(partyFlow.current.NextVariants) - 1
	if choice > paths {
		return nil, fmt.Errorf("Invalid move to %v when only %v paths are available.", choice, paths)
	}

	return partyFlow.current.NextVariants[choice].to, nil
}

func (partyFlow *PartyFlow) Start() {
	if partyFlow.start == nil {
		partyFlow.logger.Printf("%v", colors.Error("WebPartySpec not loaded."))
		return
	}

	defer func() {
		if r := recover(); r != nil {
			partyFlow.logger.Printf("PartyFlow execution panicked, unloading from memory...")
			partyFlow.start = nil
			partyFlow.current = nil
		}

		partyFlow.logger.Printf("PartyFlow finished.")
		partyFlow.onFinished()
	}()

	partyFlow.context, partyFlow.stop = context.WithCancel(context.Background())

	partyFlow.stepCounter.Store(0)

	partyFlow.current = partyFlow.start
	partyFlow.standings = make(map[string]Standing)

	partyFlow.logger.Printf("Starting from <%s>", partyFlow.current.Name)

	var path int
	moveTo := make(chan int)
	partyFlowCancelled := false

	initialWait := 1
	<-time.After(time.Duration(initialWait) * time.Second)

	for {
		partyFlow.stepCounter.Add(1)

		partyFlow.current.Step = int(partyFlow.stepCounter.Load())
		partyFlow.logger.Printf("%d | Waiting on <%s>", partyFlow.current.Step, partyFlow.current.Name)

		partyFlow.onQuery(partyFlow.current)

		partyFlow.current.setMoveToNilIfNoVariants()
		ctx, cancelOtherConditions := context.WithCancel(context.Background())

		for moveToVariant, conditionalMove := range partyFlow.current.NextVariants {
			for condition := range conditionalMove.when {
				_, ok := partyFlow.conditionCheckers[condition]

				if !ok {
					log.Panicf("%v <%s> %v",
						colors.Error("Condition function"), condition, colors.Error("is not found"))
				}

				go func(ctx context.Context) {
					select {
					case <-ctx.Done():
					case <-partyFlow.context.Done():
						partyFlowCancelled = true
						moveTo <- -1
					case <-partyFlow.conditionCheckers[condition](
						conditionalMove.when[condition],
						partyFlow.conditionArgs[condition]):
						moveTo <- moveToVariant
					}
				}(ctx)
			}
		}

		path = <-moveTo
		cancelOtherConditions()

		for len(moveTo) > 0 {
			<-moveTo
		}

		if partyFlowCancelled {
			break
		}

		var nextQuery *PartyQuery

		if partyFlow.skipGetWinners {
			partyFlow.skipGetWinners = false
		} else {
			<-time.After(200 * time.Millisecond)
			winners := partyFlow.onGetWinners(partyFlow.current)
			inputs := partyFlow.onGetInputs(partyFlow.current)

			for _, winner := range winners {
				_, ok := partyFlow.standings[winner]

				if !ok {
					partyFlow.standings[winner] = Standing{
						WinCount: 0, LastInput: inputs[winner]}
				}

				partyFlow.standings[winner] = Standing{
					WinCount: partyFlow.standings[winner].WinCount + 1, LastInput: inputs[winner]}
			}

			partyFlow.logger.Printf("Winners -> %v", partyFlow.standings)
		}

		correct, hasCorrect := partyFlow.current.Input["correct"]
		votingQueried := false

		if hasCorrect {
			correct = correct.(string)
			votingQueried = correct == "vote"
		}

		overviewerQueried := partyFlow.current.Overviewer != nil
		next, err := partyFlow.next(path)

		if err != nil {
			partyFlow.logger.Panicf("%s", err.Error())
		}

		if votingQueried {
			moveWhen := make(map[string]any)

			for key, value := range partyFlow.current.Vote {
				if key != "type" {
					moveWhen[key] = value
				}
			}

			nextQuery = &PartyQuery{
				Name:   fmt.Sprintf("%s (voting)", partyFlow.current.Name),
				Layout: nil,
				Input: map[string]any{"type": "vote " + partyFlow.current.Vote["type"].(string),
					"candidates": partyFlow.onGetInputs(partyFlow.current)},
				Overviewer:   partyFlow.current.Overviewer,
				NextVariants: []conditionalMove{{to: next, when: moveWhen}},
			}
		} else if overviewerQueried {
			moveWhen := make(map[string]any)

			for key, value := range partyFlow.current.Overviewer {
				if key != "type" {
					moveWhen[key] = value
				}
			}

			nextQuery = &PartyQuery{
				Name: fmt.Sprintf("%s (overviewer)", partyFlow.current.Name),
				Layout: map[string]any{"type": "overviewer " + partyFlow.current.Overviewer["type"].(string),
					"winners": partyFlow.standings},
				Input:        nil,
				Overviewer:   nil,
				NextVariants: []conditionalMove{{to: next, when: moveWhen}},
			}

			partyFlow.skipGetWinners = true
		} else {
			next, _ := partyFlow.next(path)

			if next == nil {
				partyFlow.logger.Panicf("%v <%v>.", colors.Error(
					"End reached unexpectedly on query "), partyFlow.current.Name)
			} else if partyFlow.current.NextVariants[0].to.Name == "end" {
				break
			}

			partyFlow.logger.Printf("<%s> --> <%s>",
				partyFlow.current.Name, partyFlow.current.NextVariants[path].to.Name)
			nextQuery, _ = partyFlow.next(path)
		}

		partyFlow.current = nextQuery
		partyFlow.onMove()
	}
}

func (partyFlow *PartyFlow) Stop() error {
	if partyFlow.stop == nil {
		return errors.New("Calling Stop() on unintialized PartyFlow. Was this intended?")
	}

	partyFlow.stop()
	return nil
}

func (partyQuery *PartyQuery) setMoveToNilIfNoVariants() {
	if partyQuery.NextVariants == nil {
		partyQuery.NextVariants = []conditionalMove{{
			to:   nil,
			when: map[string]any{"timer": int64(0)},
		}}
	}
}

func (partyFlow *PartyFlow) GetStep() int {
	return int(partyFlow.stepCounter.Load())
}

func (partyFlow *PartyFlow) OnGetWinners(cb func(*PartyQuery) []string) {
	partyFlow.onGetWinners = cb
}

func (partyFlow *PartyFlow) OnGetInputs(getInputs func(*PartyQuery) map[string]string) {
	partyFlow.onGetInputs = getInputs
}

func (partyFlow *PartyFlow) OnQuery(cb func(*PartyQuery)) {
	partyFlow.onQuery = cb
}

func (partyFlow *PartyFlow) OnMove(function func()) {
	partyFlow.onMove = function
}

func (partyFLow *PartyFlow) OnFinished(cb func()) {
	partyFLow.onFinished = cb
}

func (partyFlow *PartyFlow) AddInputChecker(name string, checker input.Checker) {
	partyFlow.inputCheckers[name] = checker
}

func (partyFlow *PartyFlow) GetInputChecker(name string) input.Checker {
	return partyFlow.inputCheckers[name]
}

func (partyFlow *PartyFlow) AddCondition(
	name string,
	channelSetter func(any, map[string]any) <-chan struct{},
	args map[string]any) {
	partyFlow.conditionCheckers[name] = channelSetter
	partyFlow.conditionArgs[name] = args
}
