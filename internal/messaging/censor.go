package messaging

import "strings"

// censor replaces every denied code point with '_' (ADR-005). Immutable after
// construction, so safe for concurrent use.
type censor struct {
	denied map[rune]struct{}
}

// NewCensor builds a Censor from the denylist string (CENSOR_CHARS); each code
// point in chars is one denied character. Empty list = identity.
func NewCensor(chars string) Censor {
	c := &censor{denied: make(map[rune]struct{})}
	for _, r := range chars {
		c.denied[r] = struct{}{}
	}
	return c
}

// Apply implements Censor.
func (c *censor) Apply(s string) string {
	if len(c.denied) == 0 {
		return s
	}
	return strings.Map(func(r rune) rune {
		if _, ok := c.denied[r]; ok {
			return '_'
		}
		return r
	}, s)
}
