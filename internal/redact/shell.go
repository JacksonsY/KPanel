package redact

import "strings"

// Only lexical boundaries are observed. No expansion or command is evaluated.
// Unsupported backticks and excessive nesting keep the sensitive region closed.
type shellFrame struct {
	quote  byte
	ansi   bool
	word   bool
	parens int
}
type shellLexicalState struct {
	frames    []shellFrame
	depth     int
	joined    bool
	uncertain bool
}

func (s *shellLexicalState) active() bool {
	return s.uncertain || s.depth != 0 || len(s.frames) > 0 && s.frames[0].quote != 0 || s.joined
}

func (s *shellLexicalState) observe(value string) {
	if len(s.frames) == 0 {
		s.frames = append(s.frames, shellFrame{})
	}
	s.joined = false
	for i := 0; i < len(value); i++ {
		f := &s.frames[s.depth]
		b := value[i]
		if f.quote == '\'' {
			if f.ansi && b == '\\' {
				if i+1 == len(value) {
					s.joined = true
					return
				}
				i++
				continue
			}
			if b == '\'' {
				f.quote = 0
				f.ansi = false
			}
			continue
		}
		if b == '\\' {
			if i+1 == len(value) {
				s.joined = true
				return
			}
			i++
			f.word = true
			continue
		}
		if b == '`' {
			s.uncertain = true
		}
		if b == '$' && i+1 < len(value) && value[i+1] == '(' {
			if s.depth == maxProjectionDepth {
				s.uncertain = true
				return
			}
			f.word = true
			s.depth++
			if s.depth == len(s.frames) {
				s.frames = append(s.frames, shellFrame{})
			}
			s.frames[s.depth] = shellFrame{parens: 1}
			i++
			continue
		}
		if f.quote == '"' {
			if b == '"' {
				f.quote = 0
			}
			continue
		}
		if b == '#' && !f.word {
			return
		}
		if b == '$' && i+1 < len(value) && value[i+1] == '\'' {
			f.quote, f.ansi, f.word = '\'', true, true
			i++
			continue
		}
		if s.depth > 0 {
			if b == '(' {
				f.parens++
			}
			if b == ')' {
				f.parens--
				if f.parens == 0 {
					s.depth--
					continue
				}
			}
		}
		if b == '\'' || b == '"' {
			f.quote, f.word = b, true
		} else if b == ' ' || b == '\t' || strings.ContainsRune(";|&()", rune(b)) {
			f.word = false
		} else {
			f.word = true
		}
	}
	if s.frames[s.depth].quote == 0 {
		s.frames[s.depth].word = false
	}
}

type logicalKey struct{ channel, layer int }
type logicalRecord struct {
	text    strings.Builder
	indices []int
	state   shellLexicalState
}

// Join only source-layer backslash continuations. The original sources remain
// authoritative for PEM and quote state; a sensitive logical record hides all
// physical fragments, including fragments before its identifier became known.
func (p *projection) logicalRecords() (map[int]string, []bool, error) {
	final := make(map[int]string)
	hidden := make([]bool, len(p.sources))
	pending := make(map[logicalKey]*logicalRecord)
	for index, source := range p.sources {
		if !source.leaf {
			continue
		}
		key := logicalKey{source.channel, source.layer}
		r := pending[key]
		if r == nil {
			if !strings.HasSuffix(source.value, "\\") {
				continue
			}
			r = &logicalRecord{}
		}
		r.state.observe(source.value)
		if len(r.indices) == 0 && !r.state.joined {
			continue
		}
		if err := p.budget.consume(len(source.value), 1); err != nil {
			return nil, nil, err
		}
		part := source.value
		if r.state.joined {
			part = part[:len(part)-1]
		}
		r.text.WriteString(part)
		r.indices = append(r.indices, index)
		if r.state.joined {
			pending[key] = r
			continue
		}
		value := r.text.String()
		if redactLine(value) != value {
			final[index] = value
			for _, previous := range r.indices {
				hidden[previous] = true
			}
		}
		delete(pending, key)
	}
	// A bounded window may stop before the next physical record. Its joined
	// identifier still has to be classified before any fragment is displayed.
	for _, r := range pending {
		value := r.text.String()
		if redactLine(value) != value {
			for _, index := range r.indices {
				hidden[index] = true
			}
		}
	}
	return final, hidden, nil
}
