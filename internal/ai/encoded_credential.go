package ai

import (
	"encoding/json"
	"errors"
	"strings"
)

var errCredentialProjectionBudget = errors.New("AI credential projection exceeds the processing budget")

// Ordinary prose still streams through the literal matcher. A JSON string is
// withheld until its source token can be decoded; it must not disclose a key in
// a reversible representation before the final completion arrives.
type encodedCredentialFilter struct {
	literal  *credentialFilter
	token    strings.Builder
	escaped  bool
	budget   credentialProjectionBudget
	output   int
	limit    int
	redacted bool
}

type credentialProjectionBudget struct{ work, nodes, limit int }

func newEncodedCredentialFilter(key string) *encodedCredentialFilter {
	return newEncodedCredentialFilterLimit(key, MaxAssistantBytes)
}

func newEncodedCredentialFilterLimit(key string, limit int) *encodedCredentialFilter {
	return &encodedCredentialFilter{literal: newCredentialFilter(key), limit: limit,
		budget: credentialProjectionBudget{nodes: 262144, limit: limit}}
}

func (b *credentialProjectionBudget) consume(size, depth int) error {
	b.nodes--
	if depth > 32 || b.nodes < 0 || size < 0 || size > b.limit || b.work > (64<<20)-size {
		return errCredentialProjectionBudget
	}
	b.work += size
	return nil
}

func (f *encodedCredentialFilter) append(out *strings.Builder, value string) error {
	value = f.literal.write(value)
	if len(value) > f.limit-f.output {
		return errCredentialProjectionBudget
	}
	f.output += len(value)
	out.WriteString(value)
	f.redacted = f.redacted || f.literal.redacted
	return nil
}

func (f *encodedCredentialFilter) write(value string) (string, error) {
	if f.literal.key == "" {
		return value, nil
	}
	var out strings.Builder
	for len(value) > 0 {
		if f.token.Len() == 0 {
			index := strings.IndexByte(value, '"')
			if index < 0 {
				if err := f.append(&out, value); err != nil {
					return "", err
				}
				return out.String(), nil
			}
			if err := f.append(&out, value[:index]); err != nil {
				return "", err
			}
			f.token.WriteByte('"')
			f.escaped = false
			value = value[index+1:]
			continue
		}
		b := value[0]
		value = value[1:]
		if f.token.Len() >= f.limit {
			return "", errCredentialProjectionBudget
		}
		f.token.WriteByte(b)
		if f.escaped {
			f.escaped = false
		} else if b == '\\' {
			f.escaped = true
		} else if b == '"' {
			projected, changed, err := projectCredentialToken(f.token.String(), true, f.literal.key, &f.budget, 0)
			f.token.Reset()
			if err != nil {
				return "", err
			}
			f.redacted = f.redacted || changed
			if err := f.append(&out, projected); err != nil {
				return "", err
			}
		}
	}
	return out.String(), nil
}

func (f *encodedCredentialFilter) finish() (string, error) {
	var out strings.Builder
	if f.token.Len() > 0 {
		projected, changed, err := projectCredentialToken(f.token.String(), false, f.literal.key, &f.budget, 0)
		f.token.Reset()
		if err != nil {
			return "", err
		}
		f.redacted = f.redacted || changed
		if err := f.append(&out, projected); err != nil {
			return "", err
		}
	}
	tail := f.literal.finish()
	if len(tail) > f.limit-f.output {
		return "", errCredentialProjectionBudget
	}
	f.output += len(tail)
	out.WriteString(tail)
	f.redacted = f.redacted || f.literal.redacted
	return out.String(), nil
}

func replaceKnownCredential(value, key string, limit int) (string, bool, error) {
	count := strings.Count(value, key)
	if count == 0 {
		return value, false, nil
	}
	marker := credentialReplacement(key)
	size := int64(len(value)) + int64(count)*int64(len(marker)-len(key))
	if size > int64(limit) {
		return "", false, errCredentialProjectionBudget
	}
	return strings.ReplaceAll(value, key, marker), true, nil
}

// Decode only string tokens with the standard JSON decoder. Nested strings may
// themselves contain encoded JSON/JSONL; recurse with one shared work budget.
// No generic password policy is applied to ordinary assistant conversation.
func projectCredentialToken(raw string, complete bool, key string, budget *credentialProjectionBudget, depth int) (string, bool, error) {
	if err := budget.consume(len(raw), depth); err != nil {
		return "", false, err
	}
	if !strings.ContainsRune(raw, '\\') {
		// Without JSON escapes the string cannot contain a deeper quoted
		// representation. Literal matching is sufficient; avoid a decoder
		// allocation for every ordinary field name or empty string.
		if err := budget.consume(0, depth+1); err != nil {
			return "", false, err
		}
		return replaceKnownCredential(raw, key, budget.limit)
	}
	source := raw
	trailingSlash := false
	if !complete {
		backslashes := 0
		for i := len(raw) - 1; i >= 0 && raw[i] == '\\'; i-- {
			backslashes++
		}
		trailingSlash = backslashes%2 != 0
		if trailingSlash {
			source = source[:len(source)-1]
		}
		source += `"`
	}
	var decoded string
	if json.Unmarshal([]byte(source), &decoded) != nil {
		return replaceKnownCredential(raw, key, budget.limit)
	}
	projected, changed, err := projectCredentialText(decoded, key, budget, depth+1)
	if err != nil || !changed {
		return raw, false, err
	}
	encoded, err := marshalProviderJSON(projected, budget.limit)
	if err != nil {
		return "", false, err
	}
	if !complete {
		encoded = encoded[:len(encoded)-1]
		if trailingSlash {
			encoded = append(encoded, '\\')
		}
	}
	return string(encoded), true, nil
}

func projectCredentialText(value, key string, budget *credentialProjectionBudget, depth int) (string, bool, error) {
	if err := budget.consume(len(value), depth); err != nil {
		return "", false, err
	}
	var out strings.Builder
	changed := false
	for start := 0; start < len(value); {
		index := strings.IndexByte(value[start:], '"')
		if index < 0 {
			if len(value)-start > budget.limit-out.Len() {
				return "", false, errCredentialProjectionBudget
			}
			out.WriteString(value[start:])
			break
		}
		index += start
		if index-start > budget.limit-out.Len() {
			return "", false, errCredentialProjectionBudget
		}
		out.WriteString(value[start:index])
		end, escaped, complete := index+1, false, false
		for end < len(value) {
			b := value[end]
			end++
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				complete = true
				break
			}
		}
		part, replaced, err := projectCredentialToken(value[index:end], complete, key, budget, depth+1)
		if err != nil {
			return "", false, err
		}
		if len(part) > budget.limit-out.Len() {
			return "", false, errCredentialProjectionBudget
		}
		out.WriteString(part)
		changed = changed || replaced
		start = end
	}
	projected, replaced, err := replaceKnownCredential(out.String(), key, budget.limit)
	return projected, changed || replaced, err
}

// Migrations and structured argument checks use the exact same representation
// policy as live streaming. Historical copies are read one bounded row at a time.
func knownCredentialText(value, key string) (string, bool, error) {
	if key == "" {
		return value, false, nil
	}
	if len(value) > 2<<20 {
		return "", false, errCredentialProjectionBudget
	}
	f := newEncodedCredentialFilterLimit(key, 2<<20)
	first, err := f.write(value)
	if err != nil {
		return "", false, err
	}
	last, err := f.finish()
	return first + last, f.redacted, err
}
