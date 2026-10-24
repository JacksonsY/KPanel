package ai

import (
	"encoding"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxProviderNativeBytes  = 1 << 20
	maxProviderRequestBytes = 16 << 20
)

var errProviderJSONBudget = errors.New("AI provider JSON exceeds the processing budget")

// encoding/json buffers an entire value even when its writer is limited. Count
// the supported protocol payload shapes before calling it, including HTML and
// control-character escaping. Custom marshalers are deliberately not invoked.
func marshalProviderJSON(value any, limit int) ([]byte, error) {
	if err := checkProviderJSONBudget(value, limit); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func checkProviderJSONBudget(value any, limit int) error {
	_, err := providerJSONSize(value, limit)
	return err
}

func providerJSONSize(value any, limit int) (int, error) {
	budget := jsonSizeBudget{remaining: limit, nodes: 262144}
	err := budget.value(reflect.ValueOf(value), 0)
	return limit - budget.remaining, err
}

type jsonSizeBudget struct{ remaining, nodes int }

func (b *jsonSizeBudget) charge(size int) error {
	if size < 0 || size > b.remaining {
		return errProviderJSONBudget
	}
	b.remaining -= size
	return nil
}

func (b *jsonSizeBudget) text(value string) error {
	if err := b.charge(2); err != nil {
		return err
	}
	for i := 0; i < len(value); {
		c := value[i]
		size, width := 1, 1
		switch {
		case c == '"' || c == '\\' || c == '\b' || c == '\f' || c == '\n' || c == '\r' || c == '\t':
			size = 2
		case c < 0x20 || c == '<' || c == '>' || c == '&':
			size = 6
		case c >= utf8.RuneSelf:
			r, n := utf8.DecodeRuneInString(value[i:])
			if r == utf8.RuneError && n == 1 {
				return errors.New("invalid UTF-8 in AI provider JSON text")
			}
			width, size = n, n
			if r == '\u2028' || r == '\u2029' {
				size = 6
			}
		}
		if err := b.charge(size); err != nil {
			return err
		}
		i += width
	}
	return nil
}

var marshalerType = reflect.TypeFor[json.Marshaler]()
var textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()

func (b *jsonSizeBudget) value(v reflect.Value, depth int) error {
	b.nodes--
	if depth > 64 || b.nodes < 0 {
		return errProviderJSONBudget
	}
	if !v.IsValid() {
		return b.charge(4)
	}
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return b.charge(4)
		}
		return b.value(v.Elem(), depth+1)
	}
	if v.Type() == reflect.TypeFor[json.RawMessage]() {
		raw := v.Bytes()
		if raw == nil {
			return b.charge(4)
		}
		if err := b.charge(len(raw)); err != nil {
			return err
		}
		// RawMessage is compacted and HTML-escaped by encoding/json. Whitespace
		// removal can only reduce this conservative reservation.
		for _, c := range raw {
			if c == '<' || c == '>' || c == '&' {
				if err := b.charge(5); err != nil {
					return err
				}
			}
		}
		for i := 0; i+2 < len(raw); i++ {
			if raw[i] == 0xe2 && raw[i+1] == 0x80 && (raw[i+2] == 0xa8 || raw[i+2] == 0xa9) {
				if err := b.charge(3); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if v.Type() == reflect.TypeFor[json.Number]() {
		return b.charge(len(v.String()))
	}
	if v.Type().Implements(marshalerType) || reflect.PointerTo(v.Type()).Implements(marshalerType) ||
		v.Type().Implements(textMarshalerType) || reflect.PointerTo(v.Type()).Implements(textMarshalerType) {
		return errors.New("unsupported custom AI provider JSON marshaler")
	}
	switch v.Kind() {
	case reflect.String:
		return b.text(v.String())
	case reflect.Bool:
		if v.Bool() {
			return b.charge(4)
		}
		return b.charge(5)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return b.charge(len(strconv.FormatInt(v.Int(), 10)))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return b.charge(len(strconv.FormatUint(v.Uint(), 10)))
	case reflect.Float32, reflect.Float64:
		f := v.Float()
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return errors.New("invalid AI provider JSON number")
		}
		format, bits, absolute := byte('f'), v.Type().Bits(), math.Abs(f)
		if absolute != 0 && ((bits == 64 && (absolute < 1e-6 || absolute >= 1e21)) || (bits == 32 && (float32(absolute) < 1e-6 || float32(absolute) >= 1e21))) {
			format = 'e'
		}
		var scratch [64]byte
		encoded := strconv.AppendFloat(scratch[:0], f, format, -1, bits)
		size := len(encoded)
		if format == 'e' && size >= 4 && encoded[size-4] == 'e' && encoded[size-3] == '-' && encoded[size-2] == '0' {
			size--
		}
		return b.charge(size)
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			return b.charge(4)
		}
		if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
			// []byte uses base64, whose alphabet requires no JSON escaping.
			if v.Len() > b.remaining {
				return errProviderJSONBudget
			}
			return b.charge(2 + (v.Len()+2)/3*4)
		}
		if err := b.charge(2 + max(0, v.Len()-1)); err != nil {
			return err
		}
		for i := 0; i < v.Len(); i++ {
			if err := b.value(v.Index(i), depth+1); err != nil {
				return err
			}
		}
	case reflect.Map:
		if v.IsNil() {
			return b.charge(4)
		}
		if v.Type().Key().Kind() != reflect.String {
			return errors.New("unsupported AI provider JSON map key")
		}
		if err := b.charge(2 + max(0, v.Len()-1)); err != nil {
			return err
		}
		iter := v.MapRange()
		for iter.Next() {
			if err := b.text(iter.Key().String()); err != nil {
				return err
			}
			if err := b.charge(1); err != nil {
				return err
			}
			if err := b.value(iter.Value(), depth+1); err != nil {
				return err
			}
		}
	case reflect.Struct:
		if err := b.charge(2); err != nil {
			return err
		}
		fields := 0
		for i := 0; i < v.NumField(); i++ {
			field := v.Type().Field(i)
			if !field.IsExported() {
				continue
			}
			if field.Anonymous {
				return errors.New("unsupported embedded AI provider JSON field")
			}
			tag := strings.Split(field.Tag.Get("json"), ",")
			if tag[0] == "-" {
				continue
			}
			omit := false
			for _, option := range tag[1:] {
				if option != "omitempty" {
					return errors.New("unsupported AI provider JSON option")
				}
				omit = jsonEmpty(v.Field(i))
			}
			if omit {
				continue
			}
			name := tag[0]
			if name == "" {
				name = field.Name
			}
			if err := b.text(name); err != nil {
				return err
			}
			if err := b.charge(1 + min(fields, 1)); err != nil {
				return err
			}
			fields++
			if err := b.value(v.Field(i), depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("unsupported AI provider JSON value")
	}
	return nil
}

func jsonEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Interface, reflect.Pointer:
		return v.IsZero()
	}
	return false
}
