package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joshchen-dev/raidy/internal/postgres"
)

func TestFailMapsAlreadyPublishedToConflict(t *testing.T) {
	h := &Handler{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	response := httptest.NewRecorder()
	h.fail(response, httptest.NewRequest(http.MethodPost, "/api/teams/1/publish", nil), postgres.ErrAlreadyPublished)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
	if !strings.Contains(response.Body.String(), "Republish") {
		t.Fatalf("body = %s, want guidance to use Republish", response.Body.String())
	}
}
