package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func getIndex(t *testing.T, name string) string {
	t.Helper()
	s := NewServer(Deps{InstanceName: name})
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
