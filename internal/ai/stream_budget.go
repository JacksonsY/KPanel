package ai

import (
	"encoding/json"
	"errors"
	"strings"
)

// Wire bytes include ignored events, repeated replacements and hidden context.
// Charge before decoding or retaining a fragment, independently of the caller.
const (
	maxSSEEventBytes    = 2 << 20
	maxSSEStreamBytes   = 8 << 20
	maxToolNameBytes    = 256
	maxToolIDBytes      = 256
	maxDiscoveredModels = 5000
)

func consumeBytes(used *int, next, limit int) error {
	if next < 0 || *used > limit-next {
		return errors.New("AI provider output exceeds the response budget")
	}
	*used += next
	return nil
}

// updateTool checks every protocol's fragments before append/copy/allocation.
func updateTool(parts map[int]*ToolCall, index int, id, name, arguments string, appendName, appendArguments bool) error {
	current := parts[index]
	if index < 0 || (current == nil && len(parts) >= MaxToolCalls) {
		return errors.New("AI provider exceeds the tool call budget")
	}
	nameBytes, argumentBytes := len(name), len(arguments)
	if current != nil {
		if appendName {
			nameBytes += len(current.Name)
		}
		if appendArguments {
			argumentBytes += len(current.Arguments)
		}
	}
	if len(id) > maxToolIDBytes || nameBytes > maxToolNameBytes || argumentBytes > MaxToolResultBytes {
		return errors.New("AI provider exceeds the tool fragment budget")
	}
	if current == nil {
		current = &ToolCall{}
		parts[index] = current
	}
	if id != "" {
		current.ID = id
	}
	if appendName {
		current.Name += name
	} else if name != "" {
		current.Name = name
	}
	if appendArguments {
		current.Arguments = append(current.Arguments, arguments...)
	} else if arguments != "" {
		current.Arguments = json.RawMessage(arguments)
	}
	return nil
}

func boundedCompletion(emit func(CompletionEvent) error) func(CompletionEvent) error {
	used := 0
	return func(event CompletionEvent) error {
		if event.err != nil {
			return event.err
		}
		if err := consumeBytes(&used, len(event.Delta), MaxAssistantBytes); err != nil {
			return err
		}
		if len(event.ToolCalls) > MaxToolCalls {
			return errors.New("AI provider exceeds the tool call budget")
		}
		privateBytes := 0
		for _, call := range event.ToolCalls {
			if len(call.Name) > maxToolNameBytes || len(call.ID) > maxToolIDBytes || len(call.Arguments) > MaxToolResultBytes || len(call.ProviderData) > maxProviderNativeBytes {
				return errors.New("AI provider exceeds the tool fragment budget")
			}
			if err := consumeBytes(&privateBytes, len(call.Arguments)+len(call.ProviderData), maxContextPrivateBytes); err != nil {
				return err
			}
		}
		return emit(event)
	}
}

var errSSEComplete = errors.New("AI SSE complete")

// Retain fragmented native text with amortized growth. A byte cap alone does
// not bound the quadratic copying from repeated string concatenation.
type anthropicTextParts struct {
	text, thinking, signature strings.Builder
}

func finishSSE(err error) error {
	if err != nil {
		return err
	}
	return errSSEComplete
}
