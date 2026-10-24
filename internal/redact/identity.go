package redact

import (
	"sort"
	"strings"
)

type literalSpan struct{ start, end int }

// Structured credentials use the same terminal/newline normalization as text
// leaves, without applying any field heuristics or mutating source provenance.
func (p *projection) identityJSON(value any, depth int) (any, error) {
	if depth > maxProjectionDepth {
		return nil, errProjectionLimit
	}
	if err := p.budget.consume(0, 1); err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case string:
		if err := p.budget.consume(len(v), 0); err != nil {
			return nil, err
		}
		return normalizeRecordValue(v), nil
	case []any:
		if err := p.budget.consume(0, len(v)); err != nil {
			return nil, err
		}
		out := make([]any, len(v))
		for i, item := range v {
			safe, err := p.identityJSON(item, depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = safe
		}
		return out, nil
	case map[string]any:
		if err := p.budget.consume(0, len(v)); err != nil {
			return nil, err
		}
		out := make(map[string]any, len(v))
		for key, item := range v {
			if err := p.budget.consume(len(key), 0); err != nil {
				return nil, err
			}
			safe, err := p.identityJSON(item, depth+1)
			if err != nil {
				return nil, err
			}
			out[normalizeRecordValue(key)] = safe
		}
		return out, nil
	default:
		return value, nil
	}
}

// Find every identity against the same normalized input. Merge overlaps before
// replacing anything, so one credential can never destroy another's identity.
func (p *projection) literalSpans(value string) ([]literalSpan, error) {
	if err := p.prepareLiteralForms(); err != nil {
		return nil, err
	}
	var spans []literalSpan
	for _, secret := range p.literals {
		if secret == "" {
			continue
		}
		if err := p.budget.consume(len(value), 0); err != nil {
			return nil, err
		}
		for start := 0; start <= len(value)-len(secret); {
			at := strings.Index(value[start:], secret)
			if at < 0 {
				break
			}
			at += start
			// Bound dense overlapping matches as well as scanning and output.
			if err := p.budget.consume(len(secret), 1); err != nil {
				return nil, err
			}
			spans = append(spans, literalSpan{at, at + len(secret)})
			start = at + 1
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	merged := spans[:0]
	for _, span := range spans {
		if len(merged) > 0 && span.start <= merged[len(merged)-1].end {
			merged[len(merged)-1].end = max(merged[len(merged)-1].end, span.end)
		} else {
			merged = append(merged, span)
		}
	}
	return merged, nil
}

func (p *projection) replaceSpans(value string, spans []literalSpan) (string, error) {
	if len(spans) == 0 {
		return value, nil
	}
	size := len(value)
	for _, span := range spans {
		size += len("[REDACTED]") - (span.end - span.start)
	}
	if err := p.budget.consume(size, 0); err != nil {
		return "", err
	}
	var out strings.Builder
	out.Grow(size)
	start := 0
	for _, span := range spans {
		out.WriteString(value[start:span.start])
		out.WriteString("[REDACTED]")
		start = span.end
	}
	out.WriteString(value[start:])
	return out.String(), nil
}

// Match identities once, after all encoded text has become ordered semantic
// leaves, before any field heuristic or replacement can alter their contents.
// Object fields remain independent channels; arrays and records keep order.
func (p *projection) projectIdentities() error {
	if err := p.prepareLiteralForms(); err != nil {
		return err
	}
	if len(p.literals) == 0 {
		return nil
	}
	type channelText struct {
		text  strings.Builder
		lines []int
	}
	channels := make(map[int]*channelText)
	for i, line := range p.lines {
		if err := p.budget.consume(len(line.value)+1, 1); err != nil {
			return err
		}
		group := channels[line.channel]
		if group == nil {
			group = &channelText{}
			channels[line.channel] = group
		}
		if len(group.lines) > 0 {
			group.text.WriteByte('\n')
		}
		group.text.WriteString(line.value)
		group.lines = append(group.lines, i)
	}
	for _, group := range channels {
		spans, err := p.literalSpans(group.text.String())
		if err != nil {
			return err
		}
		offset, spanIndex := 0, 0
		for _, i := range group.lines {
			line := &p.lines[i]
			for spanIndex < len(spans) && spans[spanIndex].end <= offset {
				spanIndex++
			}
			var intersections []literalSpan
			for j := spanIndex; j < len(spans) && spans[j].start < offset+len(line.value); j++ {
				intersections = append(intersections, literalSpan{max(0, spans[j].start-offset), min(len(line.value), spans[j].end-offset)})
			}
			line.identity, err = p.replaceSpans(line.value, intersections)
			if err != nil {
				return err
			}
			offset += len(line.value) + 1
		}
	}
	return nil
}
