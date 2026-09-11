package backend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueryUploadStatus(t *testing.T) {
	dummyAuth := map[string]string{
		"Expiry": "9999999999",
		"Auth":   "dummy-token",
	}

	t.Run("resume incomplete 308 with range", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Content-Range") != "bytes */1000" {
				t.Errorf("expected Content-Range bytes */1000, got %s", r.Header.Get("Content-Range"))
			}
			w.Header().Set("Range", "bytes=0-499")
			w.WriteHeader(308)
		}))
		defer ts.Close()

		api := &Api{
			client:            ts.Client(),
			authResponseCache: dummyAuth,
		}

		offset, isComplete, _, err := api.queryUploadStatus(context.Background(), ts.URL, 1000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isComplete {
			t.Fatal("expected incomplete")
		}
		if offset != 500 {
			t.Fatalf("expected offset 500, got %d", offset)
		}
	})

	t.Run("resume incomplete 308 without range header", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(308)
		}))
		defer ts.Close()

		api := &Api{
			client:            ts.Client(),
			authResponseCache: dummyAuth,
		}

		offset, isComplete, _, err := api.queryUploadStatus(context.Background(), ts.URL, 1000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isComplete {
			t.Fatal("expected incomplete")
		}
		if offset != 0 {
			t.Fatalf("expected offset 0, got %d", offset)
		}
	})
}
