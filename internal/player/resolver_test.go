package player

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// testAuth mimics cfg.SubsonicParams() shape (u/t/s/p) without a version param so
// the resolver's own v=1.13.0&c=memora can be asserted unambiguously.
const testAuth = "u=memora&t=token&s=salt&p=enc%3Aabc"

const (
	payloadSongByMBID = `{"subsonic-response":{"status":"ok","searchResult3":{"song":` +
		`[{"id":"song-1","title":"Song Title","artist":"Some Artist","album":"Some Album",` +
		`"musicBrainzId":"aaa-1","duration":215}]}}}`

	payloadSongMBIDMismatch = `{"subsonic-response":{"status":"ok","searchResult3":{"song":` +
		`[{"id":"song-wrong","title":"Nothing Alike","artist":"Nobody","album":"No Album",` +
		`"musicBrainzId":"zzz-9","duration":100}]}}}`

	payloadSongMetadata = `{"subsonic-response":{"status":"ok","searchResult3":{"song":[` +
		`{"id":"song-x","title":"Song Title","artist":"Other Artist","album":"Some Album",` +
		`"musicBrainzId":"zzz-1","duration":200},` +
		`{"id":"song-a","title":"Song Title","artist":"some artist ","album":"Some Album",` +
		`"musicBrainzId":"aaa-1","duration":215},` +
		`{"id":"song-b","title":"song title","artist":"Some Artist","album":"Some Album",` +
		`"musicBrainzId":"bbb-2","duration":180}]}}}`

	payloadSongNoMatch = `{"subsonic-response":{"status":"ok","searchResult3":{"song":[` +
		`{"id":"song-x","title":"Other Title","artist":"Other Artist","album":"Other Album",` +
		`"musicBrainzId":"zzz-1","duration":200}]}}}`

	payloadAlbumByMBID = `{"subsonic-response":{"status":"ok","searchResult3":{"album":` +
		`[{"id":"album-1","artist":"Some Artist","title":"Some Album","musicBrainzId":"bbb-1"}]}}}`

	payloadAlbumMetadata = `{"subsonic-response":{"status":"ok","searchResult3":{"album":[` +
		`{"id":"album-x","artist":"Other Artist","title":"Some Album","musicBrainzId":"zzz-1"},` +
		`{"id":"album-a","artist":" SOME ARTIST","title":"Some Album ","musicBrainzId":"bbb-1"}]}}}`

	payloadArtistName = `{"subsonic-response":{"status":"ok","searchResult3":{"artist":[` +
		`{"id":"artist-x","name":"Some Artist Tribute","musicBrainzId":"zzz-1"},` +
		`{"id":"artist-1","name":" some artist ","musicBrainzId":"ccc-1"}]}}}`

	payloadEmpty = `{"subsonic-response":{"status":"ok","searchResult3":{}}}`
)

// recordedRequest captures what the resolver actually sent to Subsonic.
type recordedRequest struct {
	path  string
	query url.Values
}

type recorder struct {
	mu       sync.Mutex
	requests []recordedRequest
}

func (rec *recorder) record(r *http.Request) recordedRequest {
	req := recordedRequest{path: r.URL.Path, query: r.URL.Query()}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.requests = append(rec.requests, req)
	return req
}

func (rec *recorder) all() []recordedRequest {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	out := make([]recordedRequest, len(rec.requests))
	copy(out, rec.requests)
	return out
}

func (rec *recorder) count() int {
	return len(rec.all())
}

// observedDeadline is the context deadline seen by the injected transport.
type observedDeadline struct {
	found     bool
	remaining time.Duration
}

// deadlineTransport records the context deadline of every outgoing request.
type deadlineTransport struct {
	base http.RoundTripper
	mu   sync.Mutex
	seen []observedDeadline
}

func (d *deadlineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	entry := observedDeadline{}
	if deadline, ok := req.Context().Deadline(); ok {
		entry = observedDeadline{found: true, remaining: time.Until(deadline)}
	}
	d.mu.Lock()
	d.seen = append(d.seen, entry)
	d.mu.Unlock()
	return d.base.RoundTrip(req)
}

func (d *deadlineTransport) all() []observedDeadline {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]observedDeadline, len(d.seen))
	copy(out, d.seen)
	return out
}

// newTestServer starts an httptest Subsonic stand-in.
func newTestServer(t *testing.T, respond func(req recordedRequest) (int, string)) (*httptest.Server, *recorder) {
	t.Helper()
	rec := new(recorder)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := rec.record(r)
		status, body := respond(req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

// startServer wires a Resolver pointing at an httptest Subsonic stand-in.
func startServer(t *testing.T, respond func(req recordedRequest) (int, string)) (*Resolver, *recorder) {
	t.Helper()
	srv, rec := newTestServer(t, respond)
	return NewResolver(srv.URL, testAuth, srv.Client()), rec
}

// failingTransport proves that no HTTP call is attempted.
type failingTransport struct{ t *testing.T }

func (f failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	f.t.Errorf("resolver made an HTTP call while Subsonic is not configured")
	return nil, errors.New("unexpected request")
}

func assertNone(t *testing.T, id *string, match string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != nil {
		t.Fatalf("expected nil id, got %q", *id)
	}
	if match != "none" {
		t.Fatalf("expected match %q, got %q", "none", match)
	}
}

// S3: without Subsonic configured every resolve returns nil id + match none
// and makes no HTTP call at all.
func TestResolveWithoutBaseURLReturnsNoneWithoutRequest(t *testing.T) {
	mbid := "aaa-1"
	resolver := NewResolver("", testAuth, &http.Client{Transport: failingTransport{t: t}})
	ctx := context.Background()

	t.Run("song with mbid", func(t *testing.T) {
		id, match, err := resolver.ResolveSong(ctx, &mbid, "Some Artist", "Song Title")
		assertNone(t, id, match, err)
	})
	t.Run("song without mbid", func(t *testing.T) {
		id, match, err := resolver.ResolveSong(ctx, nil, "Some Artist", "Song Title")
		assertNone(t, id, match, err)
	})
	t.Run("album", func(t *testing.T) {
		id, match, err := resolver.ResolveAlbum(ctx, &mbid, "Some Artist", "Some Album")
		assertNone(t, id, match, err)
	})
	t.Run("artist", func(t *testing.T) {
		id, match, err := resolver.ResolveArtist(ctx, "Some Artist")
		assertNone(t, id, match, err)
	})
}

// S4: an exact, case-insensitive musicBrainzId match wins.
func TestResolveSongByMBID(t *testing.T) {
	resolver, rec := startServer(t, func(recordedRequest) (int, string) {
		return http.StatusOK, payloadSongByMBID
	})

	mbid := "AAA-1"
	id, match, err := resolver.ResolveSong(context.Background(), &mbid, "Some Artist", "Song Title")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != "song-1" {
		t.Fatalf("expected id %q, got %v", "song-1", id)
	}
	if match != "mbid" {
		t.Fatalf("expected match %q, got %q", "mbid", match)
	}

	reqs := rec.all()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if reqs[0].path != "/rest/search3" {
		t.Fatalf("expected path %q, got %q", "/rest/search3", reqs[0].path)
	}
	expected := map[string]string{
		"query":       "AAA-1",
		"f":           "json",
		"v":           "1.13.0",
		"c":           "memora",
		"songCount":   "10",
		"albumCount":  "0",
		"artistCount": "0",
		"u":           "memora",
		"t":           "token",
		"s":           "salt",
		"p":           "enc:abc",
	}
	for key, want := range expected {
		if got := reqs[0].query.Get(key); got != want {
			t.Errorf("query param %q: expected %q, got %q", key, want, got)
		}
	}
}

// S4: normalized artist+title equality, first hit wins.
func TestResolveSongByMetadata(t *testing.T) {
	resolver, rec := startServer(t, func(recordedRequest) (int, string) {
		return http.StatusOK, payloadSongMetadata
	})

	id, match, err := resolver.ResolveSong(context.Background(), nil, "Some Artist", "Song Title")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != "song-a" {
		t.Fatalf("expected first match %q, got %v", "song-a", id)
	}
	if match != "metadata" {
		t.Fatalf("expected match %q, got %q", "metadata", match)
	}
	if got := rec.all()[0].query.Get("query"); got != "Song Title" {
		t.Fatalf("expected metadata query %q, got %q", "Song Title", got)
	}
}

// S4: a search3 hit whose musicBrainzId differs from the requested one is not an
// mbid match; the resolver falls back to normalized metadata.
func TestResolveSongMBIDMismatchFallsBackToMetadata(t *testing.T) {
	resolver, rec := startServer(t, func(req recordedRequest) (int, string) {
		if req.query.Get("query") == "aaa-1" {
			return http.StatusOK, payloadSongMBIDMismatch
		}
		return http.StatusOK, payloadSongMetadata
	})

	mbid := "aaa-1"
	id, match, err := resolver.ResolveSong(context.Background(), &mbid, "Some Artist", "Song Title")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != "song-a" {
		t.Fatalf("expected fallback id %q, got %v", "song-a", id)
	}
	if match != "metadata" {
		t.Fatalf("expected match %q, got %q", "metadata", match)
	}

	reqs := rec.all()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(reqs))
	}
	if got := reqs[0].query.Get("query"); got != "aaa-1" {
		t.Errorf("first query: expected %q, got %q", "aaa-1", got)
	}
	if got := reqs[1].query.Get("query"); got != "Song Title" {
		t.Errorf("second query: expected %q, got %q", "Song Title", got)
	}
}

// S2: no match is not an error.
func TestResolveSongNoMatchIsNotAnError(t *testing.T) {
	resolver, _ := startServer(t, func(recordedRequest) (int, string) {
		return http.StatusOK, payloadSongNoMatch
	})
	id, match, err := resolver.ResolveSong(context.Background(), nil, "Some Artist", "Song Title")
	assertNone(t, id, match, err)
}

func TestResolveAlbumByMBID(t *testing.T) {
	resolver, rec := startServer(t, func(recordedRequest) (int, string) {
		return http.StatusOK, payloadAlbumByMBID
	})

	mbid := "BBB-1"
	id, match, err := resolver.ResolveAlbum(context.Background(), &mbid, "Some Artist", "Some Album")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != "album-1" {
		t.Fatalf("expected id %q, got %v", "album-1", id)
	}
	if match != "mbid" {
		t.Fatalf("expected match %q, got %q", "mbid", match)
	}

	reqs := rec.all()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	expected := map[string]string{"query": "BBB-1", "albumCount": "10", "songCount": "0", "artistCount": "0"}
	for key, want := range expected {
		if got := reqs[0].query.Get(key); got != want {
			t.Errorf("query param %q: expected %q, got %q", key, want, got)
		}
	}
}

func TestResolveAlbumByMetadata(t *testing.T) {
	resolver, rec := startServer(t, func(recordedRequest) (int, string) {
		return http.StatusOK, payloadAlbumMetadata
	})

	id, match, err := resolver.ResolveAlbum(context.Background(), nil, "Some Artist", "Some Album")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != "album-a" {
		t.Fatalf("expected first match %q, got %v", "album-a", id)
	}
	if match != "metadata" {
		t.Fatalf("expected match %q, got %q", "metadata", match)
	}
	if got := rec.all()[0].query.Get("query"); got != "Some Album" {
		t.Fatalf("expected metadata query %q, got %q", "Some Album", got)
	}
}

func TestResolveAlbumNoMatchIsNotAnError(t *testing.T) {
	resolver, _ := startServer(t, func(recordedRequest) (int, string) {
		return http.StatusOK, payloadEmpty
	})
	id, match, err := resolver.ResolveAlbum(context.Background(), nil, "Some Artist", "Some Album")
	assertNone(t, id, match, err)
}

func TestResolveArtistByName(t *testing.T) {
	resolver, rec := startServer(t, func(recordedRequest) (int, string) {
		return http.StatusOK, payloadArtistName
	})

	id, match, err := resolver.ResolveArtist(context.Background(), "Some Artist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != "artist-1" {
		t.Fatalf("expected id %q, got %v", "artist-1", id)
	}
	if match != "metadata" {
		t.Fatalf("expected match %q, got %q", "metadata", match)
	}

	reqs := rec.all()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	expected := map[string]string{"query": "Some Artist", "artistCount": "10", "songCount": "0", "albumCount": "0"}
	for key, want := range expected {
		if got := reqs[0].query.Get(key); got != want {
			t.Errorf("query param %q: expected %q, got %q", key, want, got)
		}
	}
}

func TestResolveArtistNoMatchIsNotAnError(t *testing.T) {
	resolver, _ := startServer(t, func(recordedRequest) (int, string) {
		return http.StatusOK, payloadEmpty
	})
	id, match, err := resolver.ResolveArtist(context.Background(), "Some Artist")
	assertNone(t, id, match, err)
}

// S4: every Subsonic request carries the 10s context deadline.
func TestResolveUsesTenSecondDeadline(t *testing.T) {
	srv, rec := newTestServer(t, func(recordedRequest) (int, string) {
		return http.StatusOK, payloadArtistName
	})
	transport := &deadlineTransport{base: srv.Client().Transport}
	resolver := NewResolver(srv.URL, testAuth, &http.Client{Transport: transport})

	if _, _, err := resolver.ResolveArtist(context.Background(), "Some Artist"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.count() != 1 {
		t.Fatalf("expected 1 request, got %d", rec.count())
	}
	seen := transport.all()
	if len(seen) != 1 {
		t.Fatalf("expected 1 outgoing request, got %d", len(seen))
	}
	if !seen[0].found {
		t.Fatal("expected the Subsonic request to carry a context deadline")
	}
	if seen[0].remaining > 10*time.Second {
		t.Fatalf("expected deadline at most 10s away, got %s", seen[0].remaining)
	}
	if seen[0].remaining < 5*time.Second {
		t.Fatalf("expected deadline close to 10s away, got %s", seen[0].remaining)
	}
}

// S4: a caller deadline shorter than 10s still wins.
func TestResolveHonoursShorterCallerDeadline(t *testing.T) {
	srv, _ := newTestServer(t, func(recordedRequest) (int, string) {
		return http.StatusOK, payloadArtistName
	})
	transport := &deadlineTransport{base: srv.Client().Transport}
	resolver := NewResolver(srv.URL, testAuth, &http.Client{Transport: transport})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, _, err := resolver.ResolveArtist(ctx, "Some Artist"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	seen := transport.all()
	if len(seen) != 1 {
		t.Fatalf("expected 1 outgoing request, got %d", len(seen))
	}
	if !seen[0].found {
		t.Fatal("expected the Subsonic request to carry a context deadline")
	}
	if seen[0].remaining > 2*time.Second {
		t.Fatalf("expected the caller deadline to win, got %s", seen[0].remaining)
	}
	if seen[0].remaining < time.Second {
		t.Fatalf("expected the caller deadline to survive, got %s", seen[0].remaining)
	}
}

func TestResolveTransportErrors(t *testing.T) {
	t.Run("canceled context", func(t *testing.T) {
		resolver, _ := startServer(t, func(recordedRequest) (int, string) {
			return http.StatusOK, payloadArtistName
		})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		if _, _, err := resolver.ResolveSong(ctx, nil, "Some Artist", "Song Title"); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("unreachable server", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		baseURL := srv.URL
		srv.Close()
		resolver := NewResolver(baseURL, testAuth, &http.Client{Timeout: 2 * time.Second})

		_, _, err := resolver.ResolveSong(context.Background(), nil, "Some Artist", "Song Title")
		if err == nil {
			t.Fatal("expected an error for an unreachable Subsonic")
		}
		if !strings.Contains(err.Error(), "search3") {
			t.Fatalf("expected error to mention search3, got %v", err)
		}
	})
}

func TestResolveNonOKStatusIsAnError(t *testing.T) {
	resolver, _ := startServer(t, func(recordedRequest) (int, string) {
		return http.StatusInternalServerError, `{"subsonic-response":{"status":"failed"}}`
	})

	_, _, err := resolver.ResolveSong(context.Background(), nil, "Some Artist", "Song Title")
	if err == nil {
		t.Fatal("expected an error for a non-2xx Subsonic response")
	}
	if !strings.Contains(err.Error(), "search3") {
		t.Fatalf("expected error to mention search3, got %v", err)
	}
}

func TestResolveBadJSONIsAnError(t *testing.T) {
	resolver, _ := startServer(t, func(recordedRequest) (int, string) {
		return http.StatusOK, "not-json"
	})

	_, _, err := resolver.ResolveArtist(context.Background(), "Some Artist")
	if err == nil {
		t.Fatal("expected an error for a non-JSON Subsonic response")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Fatalf("expected a decode error, got %v", err)
	}
}
