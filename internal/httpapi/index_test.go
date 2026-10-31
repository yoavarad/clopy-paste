package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"clopy-paste/web"
)

func getIndex(t *testing.T, name string) string {
	t.Helper()
	s := NewServer(testDeps(Deps{InstanceName: name}))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	return rec.Body.String()
}

func TestHandleIndexInsertsInstanceName(t *testing.T) {
	body := getIndex(t, "side-a")
	if !strings.Contains(body, "[side-a]") {
		t.Fatalf("instance label missing")
	}
	if strings.Contains(body, instancePlaceholder) {
		t.Fatalf("placeholder left in page")
	}
}

func TestHandleIndexEscapesInstanceName(t *testing.T) {
	body := getIndex(t, `<script>alert("x")</script>&`)
	if strings.Contains(body, "<script>alert") {
		t.Fatalf("instance name not escaped")
	}
	if !strings.Contains(body, "&lt;script&gt;") || !strings.Contains(body, "&amp;") {
		t.Fatalf("escaped form missing")
	}
}

func TestHandleIndexInlinesSplitScript(t *testing.T) {
	body := getIndex(t, "side-a")
	if strings.Contains(body, splitPlaceholder) {
		t.Fatalf("split placeholder left in page")
	}
	if !strings.Contains(body, "function splitText(") {
		t.Fatalf("split.js not inlined")
	}
	if strings.Contains(body, `id="text" maxlength`) {
		t.Fatalf("textarea still has maxlength")
	}
}

func TestSplitScriptIsSafeToInline(t *testing.T) {
	for _, bad := range []string{"</script", "<!--", instancePlaceholder, splitPlaceholder} {
		if strings.Contains(strings.ToLower(web.SplitJS), strings.ToLower(bad)) {
			t.Fatalf("web/split.js contains %q", bad)
		}
	}
}
