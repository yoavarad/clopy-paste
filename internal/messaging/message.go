package messaging

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"
)

// Limits in code points (ADR-006).
const (
	MaxUserLen = 128
	MaxTextLen = 1000
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
	b, _ := json.Marshal(recordValue{User: user, Text: text})
	return b
}

// DecodeRecordValue parses a record value and applies the limits. Errors
// carry a short reason and never the value.
func DecodeRecordValue(b []byte) (user, text string, err error) {
	var raw map[string]json.RawMessage
	if json.Unmarshal(b, &raw) != nil || raw == nil {
		return "", "", errors.New("value is not a JSON object")
	}
	var fields [2]string
	for i, k := range [2]string{"user", "text"} {
		r, ok := raw[k]
		if !ok {
			return "", "", fmt.Errorf("missing field %s", k)
		}
		if json.Unmarshal(r, &fields[i]) != nil || string(r) == "null" {
			return "", "", fmt.Errorf("field %s is not a string", k)
		}
	}
	if err := ValidateUser(fields[0]); err != nil {
		return "", "", err
	}
	if err := ValidateText(fields[1]); err != nil {
		return "", "", err
	}
	return fields[0], fields[1], nil
}
