package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/kejilion/kejilion-panel/internal/redact"
)

func publicProviderText(value, key string) string {
	if key == "" {
		return value
	}
	safe := redact.TextWithSecrets(value, key)
	if replacement := credentialReplacement(key); replacement != "[REDACTED]" {
		// Existing display markers are trusted constants, not new secrets. A
		// short key overlapping the default marker must not make every startup
		// expand the same projected tool result again.
		if strings.HasPrefix(safe, "[REDACTED: ") && strings.HasSuffix(safe, "]") {
			return replacement
		}
		safe = strings.ReplaceAll(safe, "[REDACTED PRIVATE KEY]", replacement)
		safe = strings.ReplaceAll(safe, "[REDACTED]", replacement)
	}
	return safe
}

func credentialReplacement(key string) string {
	if key != "" && strings.Contains("[REDACTED]", key) {
		// The alphabets are disjoint, so a nonempty substring of the default
		// marker cannot also occur in this replacement.
		return "(hidden)"
	}
	return "[REDACTED]"
}

func modelContainsCredential(model Model, key string) bool {
	return key != "" && (strings.Contains(model.ID, key) || strings.Contains(model.ProviderID, key) || strings.Contains(model.ModelID, key) || strings.Contains(model.DisplayName, key))
}

// credentialFilter retains only a possible prefix of the known authentication
// key. KMP fallback visits each input byte a bounded number of times, including
// hostile repeated prefixes split into single-byte deltas.
type credentialFilter struct {
	key      string
	prefix   []int
	matched  int
	redacted bool
	marker   string
}

func newCredentialFilter(key string) *credentialFilter {
	f := &credentialFilter{key: key, prefix: make([]int, len(key)), marker: credentialReplacement(key)}
	for i, j := 1, 0; i < len(key); i++ {
		for j > 0 && key[i] != key[j] {
			j = f.prefix[j-1]
		}
		if key[i] == key[j] {
			j++
		}
		f.prefix[i] = j
	}
	return f
}
func (f *credentialFilter) write(value string) string {
	if f.key == "" {
		return value
	}
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		for f.matched > 0 && value[i] != f.key[f.matched] {
			next := f.prefix[f.matched-1]
			out.WriteString(f.key[:f.matched-next])
			f.matched = next
		}
		if value[i] == f.key[f.matched] {
			f.matched++
			if f.matched == len(f.key) {
				f.redacted = true
				out.WriteString(f.marker)
				f.matched = 0
			}
		} else {
			out.WriteByte(value[i])
		}
	}
	return out.String()
}
func (f *credentialFilter) finish() string { tail := f.key[:f.matched]; f.matched = 0; return tail }

func containsCredentialJSON(raw json.RawMessage, key string) bool {
	if key == "" {
		return false
	}
	_, found, err := knownCredentialText(string(raw), key)
	return found || err != nil
}

// All runtime and learning clients, including non-HTTP implementations, cross
// this boundary before text is emitted or persisted. Tool arguments are rejected
// rather than rewritten: a display projection must never change execution.
func streamPublicCompletion(ctx context.Context, client ModelClient, provider Provider, key string, request CompletionRequest, emit func(CompletionEvent) error, rejectCredential bool) error {
	filter := newEncodedCredentialFilter(key)
	checked := boundedCompletion(func(event CompletionEvent) error {
		for _, call := range event.ToolCalls {
			if key != "" && (strings.Contains(call.ID, key) || strings.Contains(call.Name, key) || containsCredentialJSON(call.Arguments, key) || strings.Contains(call.ArgumentsPreview, key) || strings.Contains(call.ResultPreview, key)) {
				return errors.New("provider authentication credential in tool call")
			}
		}
		var err error
		event.Delta, err = filter.write(event.Delta)
		if err != nil {
			return err
		}
		if event.Done {
			tail, err := filter.finish()
			if err != nil {
				return err
			}
			event.Delta += tail
		}
		if rejectCredential && filter.redacted {
			return errors.New("provider authentication credential in learning output")
		}
		return emit(event)
	})
	if err := client.Stream(ctx, provider, key, request, checked); err != nil {
		if key != "" {
			message, changed, projectionErr := knownCredentialText(err.Error(), key)
			if projectionErr != nil {
				return &providerTransportError{cause: err, message: "AI provider error exceeds the credential projection budget"}
			}
			if changed {
				return &providerTransportError{cause: err, message: message}
			}
		}
		return err
	}
	tail, err := filter.finish()
	if err != nil {
		return err
	}
	if rejectCredential && filter.redacted {
		return errors.New("provider authentication credential in learning output")
	}
	if tail != "" {
		return emit(CompletionEvent{Delta: tail})
	}
	return nil
}
