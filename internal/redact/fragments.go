package redact

import (
	"bytes"
	"encoding/json"
	"strings"
)

type fragmentSequence struct {
	values []any
	source int
}

func structuredFragmentAt(value string, i int) bool {
	if value[i] == '[' {
		return looksLikeJSON(value[i:])
	}
	if value[i] != '{' {
		return false
	}
	tail := strings.TrimLeft(value[i+1:], " \t")
	return len(tail) == 0 || tail[0] == '"' || tail[0] == '}' || strings.HasPrefix(tail, `\"`)
}

func containsStructuredFragment(value string) bool {
	for i := 0; i < len(value); i++ {
		if structuredFragmentAt(value, i) {
			return true
		}
	}
	return false
}

// Logs can prefix/suffix a JSON value or an encoded JSON string. Decode one
// bounded value at a time and retain the surrounding original lexical source.
// A malformed structured fragment fails closed rather than falling back to
// regular expressions that cannot recognize escaped JSON field names.
func (p *projection) embeddedText(value string, depth int) (any, bool, error) {
	start := -1
	for i := 0; i < len(value); i++ {
		b := value[i]
		candidate := structuredFragmentAt(value, i)
		if b == '"' {
			// Inspect a whole quoted token, including arbitrarily encoded JSON,
			// rather than guessing how many backslashes precede an inner brace.
			end := i + 1
			for end < len(value) {
				if value[end] == '\\' {
					end += 2
					continue
				}
				if value[end] == '"' {
					end++
					break
				}
				end++
			}
			end = min(end, len(value))
			if err := p.budget.consume(end-i, 1); err != nil {
				return nil, true, err
			}
			var decoded string
			if err := json.Unmarshal([]byte(value[i:end]), &decoded); err == nil {
				candidate = looksLikeJSON(decoded) || containsStructuredFragment(decoded)
				if !candidate {
					i = end - 1
					continue
				}
			} else if i+1 < len(value) && (strings.ContainsRune("{[", rune(value[i+1])) || strings.HasPrefix(value[i:], `"\"`)) {
				return nil, true, errInvalidJSON
			}
		}
		if candidate {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, false, nil
	}
	if depth >= maxProjectionDepth {
		return nil, true, errProjectionLimit
	}
	if err := p.budget.consume(len(value)-start, 1); err != nil {
		return nil, true, err
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(value[start:])))
	decoder.UseNumber()
	parsed, err := p.decodeValue(decoder, depth+1)
	if err != nil {
		return nil, true, err
	}
	end := start + int(decoder.InputOffset())
	// Classify the original lexical boundary before JSON decomposition. A
	// sensitive prefix owns its following value, including JSON punctuation;
	// known identities can also span the prefix, encoded value and suffix.
	// Such a mixed record is conservatively hidden as one display unit.
	sensitive := false
	if start > 0 {
		// A flag/assignment can end immediately before its structured value.
		// A fixed display-only token lets the existing classifier recognize
		// that incomplete prefix without evaluating any source expression.
		const probeValue = "_kpanel_projection_value_"
		if err := p.budget.consume(start+len(probeValue), 1); err != nil {
			return nil, true, err
		}
		prefixProbe := value[:start] + probeValue
		sensitive = redactLine(prefixProbe) != prefixProbe
	}
	spans, err := p.literalSpans(value)
	if err != nil {
		return nil, true, err
	}
	for _, span := range spans {
		if span.start < start && span.end > start || span.start < end && span.end > end {
			sensitive = true
		}
	}
	parentSource, parentLayer := p.source, p.layer
	p.source = len(p.sources)
	source := p.source
	p.sources = append(p.sources, lexicalSource{value: value, channel: p.channel, layer: p.layer, indent: p.indent, parent: parentSource, sensitive: sensitive, fragment: true})
	defer func() { p.source, p.layer = parentSource, parentLayer }()
	parts := make([]any, 0, 3)
	if start > 0 {
		prefix, err := p.leaf(value[:start])
		if err != nil {
			return nil, true, err
		}
		parts = append(parts, prefix)
	}
	p.layer++
	item, err := p.jsonValue(parsed, depth+1)
	p.layer = parentLayer
	if err != nil {
		return nil, true, err
	}
	parts = append(parts, encodedJSON{item, source})
	if end < len(value) {
		tail, found, err := p.embeddedText(value[end:], depth+1)
		if err != nil {
			return nil, true, err
		}
		if !found {
			tail, err = p.leaf(value[end:])
			if err != nil {
				return nil, true, err
			}
		}
		parts = append(parts, tail)
	}
	return fragmentSequence{parts, source}, true, nil
}

// Object names are independent metadata, not chronological payload records.
// They share admission budgets and identity policy, but cannot close a payload
// boundary or contaminate its state with quotes embedded in a field name.
func (p *projection) keyText(value string) (string, error) {
	if p.keyDepth >= maxProjectionDepth {
		return "", errProjectionLimit
	}
	child := newProjection(p.literals...)
	child.budget = p.budget
	child.keyDepth = p.keyDepth + 1
	child.literalFormsReady, child.literalForms = true, p.literalForms
	defer func() { p.budget = child.budget }()
	node, err := child.text(value, p.keyDepth+1)
	if err != nil {
		return "", err
	}
	if err := child.redact(); err != nil {
		return "", err
	}
	safe, _, err := child.render(node)
	if err != nil {
		return "", err
	}
	return safe.(string), nil
}
