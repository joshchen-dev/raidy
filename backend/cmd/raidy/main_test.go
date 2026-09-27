package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct{ err error }

func (p fakePinger) Ping(context.Context) error { return p.err }

func TestHealthEndpoints(t *testing.T) {
	tests := []struct {
		path string
		ping error
		want int
	}{
		{"/healthz", errors.New("database down"), http.StatusOK},
		{"/readyz", nil, http.StatusOK},
		{"/readyz", errors.New("database down"), http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		healthHandler(fakePinger{err: test.ping}).ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("%s status = %d, want %d", test.path, response.Code, test.want)
		}
	}
}
