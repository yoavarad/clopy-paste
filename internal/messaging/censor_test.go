package messaging

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCensor(t *testing.T) {
	t.Run("default-list", func(t *testing.T) {
		// Given: Censor built from '$!~'
		// When: Apply('a$b!c~d')
		// Then: 'a_b_c_d'
		if got := NewCensor("$!~").Apply("a$b!c~d"); got != "a_b_c_d" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("passthrough", func(t *testing.T) {
		// Given: Censor built from '$!~'
		// When: Apply('<b> "x" _\n')
		// Then: Output equals input
		in := "<b> \"x\" _\n"
		if got := NewCensor("$!~").Apply(in); got != in {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("multibyte-denied", func(t *testing.T) {
		// Given: Censor built from '€😀'
		// When: Apply('a€b😀')
		// Then: 'a_b_' and code-point length 4
		got := NewCensor("€😀").Apply("a€b😀")
		if got != "a_b_" || utf8.RuneCountInString(got) != 4 {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("empty-list", func(t *testing.T) {
		// Given: Censor built from ''
		// When: Apply('$!~')
		// Then: '$!~'
		if got := NewCensor("").Apply("$!~"); got != "$!~" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("length-preserved", func(t *testing.T) {
		// Given: Censor built from '$'
		// When: Apply of 1000 '$'
		// Then: 1000 '_' (code points)
		got := NewCensor("$").Apply(strings.Repeat("$", 1000))
		if got != strings.Repeat("_", 1000) {
			t.Fatalf("got %d runes", utf8.RuneCountInString(got))
		}
	})
}
