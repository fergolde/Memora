package player

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSongReturnsMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/getSong" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("id") != "song-1" {
			t.Errorf("unexpected id %s", r.URL.Query().Get("id"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"subsonic-response": map[string]any{
				"status": "ok",
				"song": map[string]any{
					"id":            "song-1",
					"title":         "Bang Bang",
					"artist":        "Santaflow",
					"album":         "Album",
					"musicBrainzId": "3bbeb4e3-ab6d-460d-bfc5-de49e4251061",
					"duration":      210,
				},
			},
		})
	}))
	defer srv.Close()

	res := NewResolver(srv.URL, "u=x", srv.Client())
	song, err := res.GetSong(context.Background(), "song-1")
	if err != nil {
		t.Fatalf("GetSong: %v", err)
	}
	if song.Title != "Bang Bang" || song.Artist != "Santaflow" {
		t.Errorf("wrong metadata %+v", song)
	}
	if song.MusicBrainzID != "3bbeb4e3-ab6d-460d-bfc5-de49e4251061" {
		t.Errorf("wrong mbid %s", song.MusicBrainzID)
	}
}

func TestGetSongUnconfiguredIsError(t *testing.T) {
	res := NewResolver("", "u=x", nil)
	if _, err := res.GetSong(context.Background(), "song-1"); err == nil {
		t.Error("expected error for unconfigured Subsonic")
	}
}

func TestGetSongNotFoundIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	res := NewResolver(srv.URL, "u=x", srv.Client())
	if _, err := res.GetSong(context.Background(), "missing"); err == nil {
		t.Error("expected error for non-2xx")
	}
}
