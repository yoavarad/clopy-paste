package messaging

import (
	"errors"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name   string
		field  string
		val    string
		max    int
		wantOK bool
	}{
		{"user 0", "user", "", MaxUserLen, false},
		{"user 1", "user", "a", MaxUserLen, true},
		{"user max", "user", strings.Repeat("a", MaxUserLen), MaxUserLen, true},
		{"user max+1", "user", strings.Repeat("a", MaxUserLen+1), MaxUserLen, false},
		{"user emoji max", "user", strings.Repeat("😀", MaxUserLen), MaxUserLen, true},
		{"user invalid utf8", "user", "\xff", MaxUserLen, false},
		{"text 0", "text", "", MaxTextLen, false},
		{"text 1", "text", "a", MaxTextLen, true},
		{"text max", "text", strings.Repeat("a", MaxTextLen), MaxTextLen, true},
		{"text max+1", "text", strings.Repeat("a", MaxTextLen+1), MaxTextLen, false},
		{"text 1000 emoji", "text", strings.Repeat("😀", MaxTextLen), MaxTextLen, true},
		{"text 1001 emoji", "text", strings.Repeat("😀", MaxTextLen+1), MaxTextLen, false},
		{"text invalid utf8", "text", "a\xffb", MaxTextLen, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var err error
			if c.field == "user" {
				err = ValidateUser(c.val)
			} else {
				err = ValidateText(c.val)
			}
			if c.wantOK {
				if err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				return
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want ValidationError, got %v", err)
			}
			if ve.Field != c.field || ve.Max != c.max {
				t.Fatalf("got field=%q max=%d", ve.Field, ve.Max)
			}
		})
	}
}

func TestEncodeRecordValue(t *testing.T) {
	got := string(EncodeRecordValue("bob", "hi \"x\""))
	want := `{"user":"bob","text":"hi \"x\""}`
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range [][2]string{{"bob", "hello_"}, {"é😀", "line\nbreak <&>"}, {strings.Repeat("a", MaxUserLen), strings.Repeat("😀", MaxTextLen)}} {
		u, x, err := DecodeRecordValue(EncodeRecordValue(tc[0], tc[1]))
		if err != nil || u != tc[0] || x != tc[1] {
			t.Fatalf("roundtrip %q: %q %q %v", tc, u, x, err)
		}
	}
}

func TestDecodeRecordValue(t *testing.T) {
	bad := map[string]string{
		"array":        `[1]`,
		"string":       `"x"`,
		"null":         `null`,
		"empty":        ``,
		"garbage":      `{`,
		"missing user": `{"text":"a"}`,
		"missing text": `{"user":"a"}`,
		"user number":  `{"user":1,"text":"a"}`,
		"text null":    `{"user":"a","text":null}`,
		"empty user":   `{"user":"","text":"a"}`,
		"long text":    `{"user":"a","text":"` + strings.Repeat("a", MaxTextLen+1) + `"}`,
		"long user":    `{"user":"` + strings.Repeat("a", MaxUserLen+1) + `","text":"a"}`,
	}
	for name, in := range bad {
		t.Run(name, func(t *testing.T) {
			_, _, err := DecodeRecordValue([]byte(in))
			if err == nil {
				t.Fatal("want error")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("leaks value")
			}
		})
	}
	u, x, err := DecodeRecordValue([]byte(`{"extra":1,"user":"a","text":"b","z":{}}`))
	if err != nil || u != "a" || x != "b" {
		t.Fatalf("unknown fields: %q %q %v", u, x, err)
	}
	_, _, err = DecodeRecordValue([]byte(`{"user":"secret","text":1}`))
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("err %v", err)
	}
}
