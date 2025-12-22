package partyflow

import (
	"fmt"
	"os"
	"testing"

	"github.com/theWebPartyTime/server/internal/conditions"
)

const test1 = `
start = "intro"

[intro]
    [intro.layout]
    type = "basic"
    title = "Угадай число"
    description = "Игра скоро начнётся"

        [intro.to.guess1]
        timer = 3

[guess1]
    [guess1.layout]
    type = "basic"
    title = "Угадай число"
    description = "Угадайте число от 1 до 5"

    [guess1.input]
    title = "Введите число от 1 до 5"
    description = "Введите вашу догадку"
    type = "text"
    correct = "vote"

        [guess1.to.end]
        timer = 3
    
    [guess1.overviewer]
    type = "winner"
    timer = 1
    
    [guess1.vote]
    type = "all"
    timer = 2

`

func TestPartyFlow(t *testing.T) {
	partyFlow, err := New().FromString("test1", test1, os.Stdout)
	if err != nil {
		fmt.Printf("%v\n", err)
	} else {
		partyFlow.AddCondition("timer", conditions.Timer, make(map[string]any))
		partyFlow.Start()
	}

}
