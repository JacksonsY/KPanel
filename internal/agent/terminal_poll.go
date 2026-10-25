package agent

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"time"
)

const (
	maxTerminalWait    = 1500 * time.Millisecond
	terminalPollPeriod = 25 * time.Millisecond
)

type terminalReadQuery struct {
	Offset          int64
	Wait            time.Duration
	InputOpen       bool
	HasInputState   bool
	ModelProjection bool
}

func parseTerminalReadQuery(values url.Values, requireOffset bool) (terminalReadQuery, error) {
	for key, entries := range values {
		if (key != "offset" && key != "wait" && key != "inputOpen" && key != "projection") || len(entries) != 1 {
			return terminalReadQuery{}, errors.New("invalid terminal query")
		}
	}
	result := terminalReadQuery{}
	if projection, ok := values["projection"]; ok {
		if projection[0] != terminalDisplayVersion {
			return result, errors.New("invalid terminal projection")
		}
		result.ModelProjection = true
	}
	rawOffset, hasOffset := values["offset"]
	if requireOffset && !hasOffset {
		return terminalReadQuery{}, errors.New("terminal offset is required")
	}
	if hasOffset {
		offset, err := strconv.ParseInt(rawOffset[0], 10, 64)
		if err != nil || offset < 0 {
			return terminalReadQuery{}, errors.New("invalid terminal offset")
		}
		result.Offset = offset
	}
	if rawWait, ok := values["wait"]; ok {
		waitMilliseconds, err := strconv.Atoi(rawWait[0])
		if err != nil || waitMilliseconds < 0 ||
			time.Duration(waitMilliseconds)*time.Millisecond > maxTerminalWait {
			return terminalReadQuery{}, errors.New("invalid terminal wait")
		}
		result.Wait = time.Duration(waitMilliseconds) * time.Millisecond
	}
	if rawInputOpen, ok := values["inputOpen"]; ok {
		inputOpen, err := strconv.ParseBool(rawInputOpen[0])
		if err != nil {
			return terminalReadQuery{}, errors.New("invalid terminal input state")
		}
		result.InputOpen = inputOpen
		result.HasInputState = true
	}
	return result, nil
}

func waitForTerminalChunk[T any](
	ctx context.Context,
	wait time.Duration,
	read func() (T, error),
	ready func(T) bool,
) (T, error) {
	value, err := read()
	if err != nil || wait <= 0 || ready(value) {
		return value, err
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	ticker := time.NewTicker(terminalPollPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return value, ctx.Err()
		case <-timer.C:
			return value, nil
		case <-ticker.C:
			value, err = read()
			if err != nil || ready(value) {
				return value, err
			}
		}
	}
}
