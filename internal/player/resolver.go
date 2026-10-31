// Package player resolves Memora catalog entities to Subsonic IDs.
//
// It is a pure package: it never reads global configuration and has no cgo or
// libvips dependency, so it can be tested anywhere.
package player

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Match kinds reported by the Resolve* methods.
const (
	// MatchMBID means the entity was found by an exact musicBrainzId match.
	MatchMBID = "mbid"
	// MatchMetadata means the entity was found by normalized artist+title equality.
	MatchMetadata = "metadata"
	// MatchNone means no Subsonic entity matched, or Subsonic is not configured.
	MatchNone = "none"
)

const (
	search3Path   = "/rest/search3"
	search3API    = "1.13.0"
	search3Client = "memora"
	search3Limit  = 10
	// defaultTimeout bounds every Subsonic request made by the resolver.
	defaultTimeout = 10 * time.Second
)

// Resolver looks up Subsonic IDs for songs, albums and artists. Its zero value
// is not usable; build one with NewResolver.
type Resolver struct {
	baseURL    string
	authParams string
	client     *http.Client
}

// NewResolver returns a Resolver for the given Subsonic base URL and auth query
// string (for example "u=user&t=token&s=salt"). An empty baseURL means Subsonic
// is not configured: every resolve then reports MatchNone without any HTTP call.
func NewResolver(baseURL, authParams string, client *http.Client) *Resolver {
	if client == nil {
		client = http.DefaultClient
	}
	return &Resolver{
		baseURL:    strings.TrimSuffix(strings.TrimSpace(baseURL), "/"),
		authParams: authParams,
		client:     client,
	}
}

// search3Response is the search3 payload subset the resolver consumes.
type search3Response struct {
	SubsonicResponse struct {
		Status        string `json:"status"`
		SearchResult3 struct {
			Song   []songItem   `json:"song"`
			Album  []albumItem  `json:"album"`
			Artist []artistItem `json:"artist"`
		} `json:"searchResult3"`
	} `json:"subsonic-response"`
}

type songItem struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Artist        string `json:"artist"`
	Album         string `json:"album"`
	MusicBrainzID string `json:"musicBrainzId"`
	Duration      int    `json:"duration"`
}

type albumItem struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Artist        string `json:"artist"`
	MusicBrainzID string `json:"musicBrainzId"`
}

type artistItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	MusicBrainzID string `json:"musicBrainzId"`
}

// ResolveSong returns the Subsonic song ID for the given musicBrainzId or, when
// that fails, for the given artist and title. A miss is not an error.
func (r *Resolver) ResolveSong(ctx context.Context, mbid *string, artist, title string) (id *string, match string, err error) {
	if r.baseURL == "" {
		return nil, MatchNone, nil
	}

	if mbid != nil && *mbid != "" {
		resp, err := r.search3(ctx, *mbid, search3Limit, 0, 0)
		if err != nil {
			return nil, MatchNone, err
		}
		for _, song := range resp.SubsonicResponse.SearchResult3.Song {
			if song.ID != "" && strings.EqualFold(song.MusicBrainzID, *mbid) {
				return ptr(song.ID), MatchMBID, nil
			}
		}
	}

	resp, err := r.search3(ctx, title, search3Limit, 0, 0)
	if err != nil {
		return nil, MatchNone, err
	}
	for _, song := range resp.SubsonicResponse.SearchResult3.Song {
		if song.ID != "" && normalize(song.Artist) == normalize(artist) && normalize(song.Title) == normalize(title) {
			return ptr(song.ID), MatchMetadata, nil
		}
	}
	return nil, MatchNone, nil
}

// ResolveAlbum returns the Subsonic album ID for the given musicBrainzId or, when
// that fails, for the given artist and album title. A miss is not an error.
func (r *Resolver) ResolveAlbum(ctx context.Context, mbid *string, artist, album string) (id *string, match string, err error) {
	if r.baseURL == "" {
		return nil, MatchNone, nil
	}

	if mbid != nil && *mbid != "" {
		resp, err := r.search3(ctx, *mbid, 0, search3Limit, 0)
		if err != nil {
			return nil, MatchNone, err
		}
		for _, al := range resp.SubsonicResponse.SearchResult3.Album {
			if al.ID != "" && strings.EqualFold(al.MusicBrainzID, *mbid) {
				return ptr(al.ID), MatchMBID, nil
			}
		}
	}

	resp, err := r.search3(ctx, album, 0, search3Limit, 0)
	if err != nil {
		return nil, MatchNone, err
	}
	for _, al := range resp.SubsonicResponse.SearchResult3.Album {
		if al.ID != "" && normalize(al.Artist) == normalize(artist) && normalize(al.Title) == normalize(album) {
			return ptr(al.ID), MatchMetadata, nil
		}
	}
	return nil, MatchNone, nil
}

// ResolveArtist returns the Subsonic artist ID for the given artist name using
// normalized name equality. A miss is not an error.
func (r *Resolver) ResolveArtist(ctx context.Context, artist string) (id *string, match string, err error) {
	if r.baseURL == "" {
		return nil, MatchNone, nil
	}

	resp, err := r.search3(ctx, artist, 0, 0, search3Limit)
	if err != nil {
		return nil, MatchNone, err
	}
	for _, ar := range resp.SubsonicResponse.SearchResult3.Artist {
		if ar.ID != "" && normalize(ar.Name) == normalize(artist) {
			return ptr(ar.ID), MatchMetadata, nil
		}
	}
	return nil, MatchNone, nil
}

// Song is the Subsonic song metadata needed to map a Subsonic ID back to
// the Memora catalog (player scrobble path).
type Song struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Artist        string `json:"artist"`
	Album         string `json:"album"`
	MusicBrainzID string `json:"musicBrainzId"`
	Duration      int    `json:"duration"`
}

// GetSong returns one Subsonic song by ID. Unlike the Resolve* methods, a
// missing song or an unconfigured server IS an error: callers need the
// metadata to continue.
func (r *Resolver) GetSong(ctx context.Context, id string) (*Song, error) {
	if r.baseURL == "" {
		return nil, fmt.Errorf("player: subsonic not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("%s/rest/getSong?%s&f=json&v=%s&c=%s&id=%s",
		r.baseURL, r.authParams, search3API, search3Client, url.QueryEscape(id))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("player: getSong request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("player: getSong request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("player: getSong status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("player: getSong read: %w", err)
	}

	var out struct {
		SubsonicResponse struct {
			Status string `json:"status"`
			Song   *Song  `json:"song"`
		} `json:"subsonic-response"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("player: getSong decode: %w", err)
	}
	if out.SubsonicResponse.Song == nil || out.SubsonicResponse.Song.ID == "" {
		return nil, fmt.Errorf("player: getSong: song not found")
	}
	return out.SubsonicResponse.Song, nil
}

// search3 performs one Subsonic search3 request bounded by defaultTimeout.
func (r *Resolver) search3(ctx context.Context, query string, songCount, albumCount, artistCount int) (*search3Response, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("%s%s?%s&f=json&query=%s&v=%s&c=%s&songCount=%d&albumCount=%d&artistCount=%d",
		r.baseURL, search3Path, r.authParams, url.QueryEscape(query), search3API, search3Client,
		songCount, albumCount, artistCount)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("player: search3 request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("player: search3 request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("player: search3 status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("player: search3 read: %w", err)
	}

	out := new(search3Response)
	if err := json.Unmarshal(body, out); err != nil {
		return nil, fmt.Errorf("player: search3 decode: %w", err)
	}
	return out, nil
}

// normalize lowercases and trims a metadata string for tolerant comparison.
func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func ptr(s string) *string {
	return &s
}
