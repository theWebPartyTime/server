package partyflow

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
)

func (partyFlow *PartyFlow) FromFile(filePath string, logWriter io.Writer) (*PartyFlow, error) {
	debugName := filePath[strings.LastIndex(filePath, "/")+1:]
	file, readErr := os.ReadFile(filePath)

	if readErr != nil {
		return nil, errors.New("Failed to read WebPartySpec file.")
	}

	return partyFlow.FromString(debugName, string(file), logWriter)
}

func (partyFlow *PartyFlow) FromString(debugName string, webPartySpec string, logWriter io.Writer) (*PartyFlow, error) {
	var generalError error = nil

	partyFlow.logger = log.New(
		logWriter,
		fmt.Sprintf("<%s> ", debugName),
		log.Ldate|log.Ltime|log.Lmsgprefix,
	)

	var webPartySpecMap map[string]any
	partyFlow.logger.Print("Loading...")

	parseErr := toml.Unmarshal([]byte(webPartySpec), &webPartySpecMap)
	if parseErr != nil {
		return nil, fmt.Errorf("Failed to parse WebPartySpec TOML:\n\t- %w", parseErr)
	}

	start, buildErr := partyFlow.parse(webPartySpecMap)
	if buildErr != nil {
		return nil, fmt.Errorf("Failed to build PartyFlow from WebPartySpec:\n\t- %w", buildErr)
	}

	partyFlow.start = start
	partyFlow.logger.Printf("Ready to start")

	return partyFlow, generalError
}

func (partyFlow *PartyFlow) parse(webPartySpec map[string]any) (*PartyQuery, error) {
	var start *PartyQuery = nil
	var parseError error = nil

	defer func() {
		if r := recover(); r != nil {
			start = nil
			parseError = errors.New("Some type casts have failed while parsing. Is WebPartySpec provided data valid?")
		}
	}()

	startQueryName := webPartySpec["start"].(string)
	var nameToQuery = map[string]*PartyQuery{"end": {Name: "end"}}

	ignoreKeys := map[string]any{"start": nil, "end": nil}
	for key, value := range webPartySpec {
		_, ignoreKey := ignoreKeys[key]

		if ignoreKey {
			continue
		}

		reflectValue := reflect.ValueOf(value)

		if reflectValue.Kind() == reflect.Map {
			if reflect.Value.Len(reflectValue) == 0 {
				return nil, fmt.Errorf("Empty PartyQuery (%s)", key)
			}
		} else {
			return nil, fmt.Errorf("Unknown parameter (%s)", key)
		}
	}

	for queryName := range webPartySpec {
		_, ignore := ignoreKeys[queryName]
		if ignore {
			continue
		}

		var query PartyQuery
		query.Name = queryName
		query.Vote = nil

		queryData := webPartySpec[queryName].(map[string]any)

		query.Layout = mapOrNil(queryData["layout"])
		query.Input = mapOrNil(queryData["input"])
		query.Overviewer = mapOrNil(queryData["overviewer"])

		if query.Overviewer != nil {
			_, overviewerTypeSpecified := query.Overviewer["type"]
			if !overviewerTypeSpecified {
				return nil, fmt.Errorf("Overviewer type unspecified (%s).", queryName)
			}

			if len(query.Overviewer) == 1 {
				return nil, fmt.Errorf("At least one move condition for overviewer should be included (%s).", queryName)
			}
		}

		_, layoutTypeSpecified := query.Layout["type"]
		if query.Layout != nil && !layoutTypeSpecified {
			return nil, fmt.Errorf("Layout type unspecified (%s).", queryName)
		}

		_, inputTypeSpecified := query.Input["type"]
		if query.Input != nil && !inputTypeSpecified {
			return nil, fmt.Errorf("Input type unspecified (%s).", queryName)
		}

		correctType, inputCorrectSpecified := query.Input["correct"]
		if query.Input != nil && !inputCorrectSpecified {
			return nil, fmt.Errorf("Input check ('correct') unspecified (%s).", queryName)
		}

		switch correctType {
		case "pick":
			_, inputLimitsExist := query.Input["limits"]
			if !inputLimitsExist {
				return nil, fmt.Errorf("Input check 'pick' used while 'limits' are unspecified (%s).", queryName)
			}
		case "vote":
			voteSection := mapOrNil(queryData["vote"])
			if voteSection == nil {
				return nil, fmt.Errorf("Input check 'vote' used while [%s.vote] is not present.", queryName)
			}

			_, votingTypeSpecified := voteSection["type"]
			if !votingTypeSpecified {
				return nil, fmt.Errorf("Voting type unspecified (%s).", queryName)
			}

			if len(voteSection) == 1 {
				return nil, fmt.Errorf("At least one move condition for voting should be included (%s).", queryName)
			}

			query.Vote = voteSection
		}

		nameToQuery[query.Name] = &query

		if query.Name == startQueryName {
			start = &query
		}
	}

	for queryName := range webPartySpec {
		_, ignore := ignoreKeys[queryName]
		if ignore {
			continue
		}

		query := nameToQuery[queryName]
		queryData := webPartySpec[queryName].(map[string]any)

		destinations := mapOrNil(queryData["to"])
		if destinations == nil {
			return nil, fmt.Errorf("Query without destination (%s)", queryName)
		}

		for destination := range destinations {
			_, destinationFound := nameToQuery[destination]
			if !destinationFound {
				return nil, fmt.Errorf("PartyQuery <%s> referenced in <%s> not found.", destination, queryName)
			}

			conditions := mapOrNil(destinations[destination])
			if conditions == nil {
				return nil, fmt.Errorf("Query without conditions: (%s)", queryName)
			}

			query.NextVariants = append(query.NextVariants, conditionalMove{
				to:   nameToQuery[destination],
				when: conditions,
			})
		}
	}

	return start, parseError
}

func mapOrNil(value any) map[string]any {
	if value != nil {
		return value.(map[string]any)
	}

	return nil
}
