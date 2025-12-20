package input

import (
	"math/rand"
)

type Checker struct {
	Pick      func(limits []any) any
	IsCorrect func(input string, correct any) bool
}

func GetTextChecker() Checker {
	return Checker{
		Pick: func(limits []any) any {
			return limits[rand.Intn(len(limits))]
		},

		IsCorrect: func(input string, correct any) bool {
			return input == correct.(string)
		},
	}
}
