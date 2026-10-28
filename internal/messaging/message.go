package messaging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// Limits in code points (ADR-006).
const (
	MaxUserLen = 128
	MaxTextLen = 256
)

// ValidationError reports a field outside 1..Max code points (or invalid UTF-8).
type ValidationError struct {
	Field string
	Max   int
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s must be 1..%d characters", e.Field, e.Max)
}

func validate(field, s string, max int) error {
	n := utf8.RuneCountInString(s)
	if n < 1 || n > max || !utf8.ValidString(s) {
		return &ValidationError{Field: field, Max: max}
	}
	return nil
}

// ValidateUser checks the username limits.
func ValidateUser(s string) error { return validate("user", s, MaxUserLen) }

// ValidateText checks the message text limits.
func ValidateText(s string) error { return validate("text", s, MaxTextLen) }

type recordValue struct {
	User string `json:"user"`
	Text string `json:"text"`
}

// EncodeRecordValue builds the Kafka record value {"user":..,"text":..} (ADR-007).
func EncodeRecordValue(user, text string) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(recordValue{User: user, Text: text})
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

// Stable reason codes for DecodeError (bad-incoming-record WARN).
const (
	ReasonInvalidJSON  = "invalid_json"
	ReasonInvalidUTF8  = "invalid_utf8"
	ReasonMissingField = "missing_field"
	ReasonWrongType    = "wrong_type"
	ReasonUserLength   = "user_length"
	ReasonTextLength   = "text_length"
)

// DecodeError is a record rejection. Reason is a stable code; the value is never included.
type DecodeError struct{ Reason string }

func (e *DecodeError) Error() string { return e.Reason }

func hexVal(b []byte) (int, bool) {
	n := 0
	for _, c := range b {
		switch {
		case c >= '0' && c <= '9':
			n = n<<4 | int(c-'0')
		case c >= 'a' && c <= 'f':
			n = n<<4 | int(c-'a'+10)
		case c >= 'A' && c <= 'F':
			n = n<<4 | int(c-'A'+10)
		default:
			return 0, false
		}
	}
	return n, true
}

// hasLoneSurrogate reports \uD800-\uDFFF escapes not forming a valid pair.
func hasLoneSurrogate(b []byte) bool {
	for i := 0; i < len(b); i++ {
		if b[i] != '\\' || i+1 >= len(b) {
			continue
		}
		if b[i+1] != 'u' {
			i++
			continue
		}
		if i+6 > len(b) {
			return false
		}
		v, ok := hexVal(b[i+2 : i+6])
		if !ok {
			return false
		}
		switch {
		case v >= 0xD800 && v < 0xDC00:
			if i+12 <= len(b) && b[i+6] == '\\' && b[i+7] == 'u' {
				if w, ok := hexVal(b[i+8 : i+12]); ok && w >= 0xDC00 && w < 0xE000 {
					i += 11
					continue
				}
			}
			return true
		case v >= 0xDC00 && v < 0xE000:
			return true
		}
		i += 5
	}
	return false
}

// DecodeRecordValue parses a record value and applies the limits. Errors are
// *DecodeError with a stable reason code and never carry the value.
func DecodeRecordValue(b []byte) (user, text string, err error) {
	if !utf8.Valid(b) || hasLoneSurrogate(b) {
		return "", "", &DecodeError{ReasonInvalidUTF8}
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(b, &raw) != nil || raw == nil {
		return "", "", &DecodeError{ReasonInvalidJSON}
	}
	var fields [2]string
	for i, k := range [2]string{"user", "text"} {
		r, ok := raw[k]
		if !ok {
			return "", "", &DecodeError{ReasonMissingField}
		}
		if string(r) == "null" || json.Unmarshal(r, &fields[i]) != nil {
			return "", "", &DecodeError{ReasonWrongType}
		}
	}
	if ValidateUser(fields[0]) != nil {
		return "", "", &DecodeError{ReasonUserLength}
	}
	if ValidateText(fields[1]) != nil {
		return "", "", &DecodeError{ReasonTextLength}
	}
	return fields[0], fields[1], nil
}
