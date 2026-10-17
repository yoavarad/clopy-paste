package httpapi

import (
	"fmt"
	"strings"
	"testing"

	"clopy-paste/internal/messaging"
)

// testDeps fills the handler deps a test leaves nil with working defaults, so NewServer's
// nil guard only fires in the tests that check it.
func testDeps(d Deps) Deps {
	if d.Censor == nil {
		d.Censor = messaging.NewCensor("")
	}
	if d.Publisher == nil {
		d.Publisher = &messaging.FakePublisher{}
	}
	if d.Store == nil {
		d.Store = messaging.NewStore()
	}
	return d
}

func TestNewServerRejectsNilDeps(t *testing.T) {
	full := testDeps(Deps{})
	cases := map[string]Deps{
		"Censor":    {Publisher: full.Publisher, Store: full.Store},
		"Publisher": {Censor: full.Censor, Store: full.Store},
		"Store":     {Censor: full.Censor, Publisher: full.Publisher},
	}
	for name, deps := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				rec := recover()
				if rec == nil || !strings.Contains(fmt.Sprint(rec), name) {
					t.Fatalf("recover() = %v, want panic naming %s", rec, name)
				}
			}()
			NewServer(deps)
		})
	}
}

func TestNewServerAcceptsFullDeps(t *testing.T) {
	if s := NewServer(testDeps(Deps{})); s == nil {
		t.Fatal("NewServer returned nil")
	}
}
