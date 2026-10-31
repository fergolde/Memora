package handlers_test

// NOTE: package engine (and therefore these tests) requires libvips via
// internal/images → bimg/cgo, so they run in CI, not on hosts without libvips.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fergolde/memora/engine/handlers"
	"github.com/fergolde/memora/internal/db/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPlayerTestDB(t *testing.T) *sqlite.Sqlite {
	t.Helper()
	s, err := sqlite.NewInMemory()
	require.NoError(t, err)
	return s
}

func TestGetPlayerResolveHandlerMissingParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/apis/web/v1/player/resolve", nil)
	rec := httptest.NewRecorder()

	handlers.GetPlayerResolveHandler()(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetPlayerResolveHandlerUnconfiguredSubsonic(t *testing.T) {
	// CI has no MEMORA_SUBSONIC_URL, so the resolver short-circuits:
	// 200 with a null subsonic_id instead of an error.
	req := httptest.NewRequest(http.MethodGet, "/apis/web/v1/player/resolve?mbid=3bbeb4e3-ab6d-460d-bfc5-de49e4251061", nil)
	rec := httptest.NewRecorder()

	handlers.GetPlayerResolveHandler()(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Type       string  `json:"type"`
		SubsonicID *string `json:"subsonic_id"`
		Match      string  `json:"match"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "song", body.Type)
	assert.Nil(t, body.SubsonicID)
	assert.Equal(t, "none", body.Match)
}

func TestGetPlayerTopTracksHandlerEmptyDB(t *testing.T) {
	store := newPlayerTestDB(t)
	req := httptest.NewRequest(http.MethodGet, "/apis/web/v1/player/top/tracks?period=all_time&limit=10", nil)
	rec := httptest.NewRecorder()

	handlers.GetPlayerTopTracksHandler(store)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Items []any `json:"items"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body.Items)
}

func TestGetPlayerRecentHandlerEmptyDB(t *testing.T) {
	store := newPlayerTestDB(t)
	req := httptest.NewRequest(http.MethodGet, "/apis/web/v1/player/recent?period=all_time&limit=10", nil)
	rec := httptest.NewRecorder()

	handlers.GetPlayerRecentHandler(store)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestGetPlayerSearchHandlerMissingQuery(t *testing.T) {
	store := newPlayerTestDB(t)
	req := httptest.NewRequest(http.MethodGet, "/apis/web/v1/player/search", nil)
	rec := httptest.NewRecorder()

	handlers.GetPlayerSearchHandler(store)(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetPlayerTopAlbumsHandlerEmptyDB(t *testing.T) {
	store := newPlayerTestDB(t)
	req := httptest.NewRequest(http.MethodGet, "/apis/web/v1/player/top/albums?period=all_time&limit=10", nil)
	rec := httptest.NewRecorder()

	handlers.GetPlayerTopAlbumsHandler(store)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestGetPlayerRecommendationsHandlerEmptyDB(t *testing.T) {
	store := newPlayerTestDB(t)
	req := httptest.NewRequest(http.MethodGet, "/apis/web/v1/player/recommendations?limit=10", nil)
	rec := httptest.NewRecorder()

	handlers.GetPlayerRecommendationsHandler(store)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Tracks []any `json:"tracks"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body.Tracks)
}

func TestGetPlayerRadioHandlerMissingSeed(t *testing.T) {
	store := newPlayerTestDB(t)
	req := httptest.NewRequest(http.MethodGet, "/apis/web/v1/player/radio", nil)
	rec := httptest.NewRecorder()

	handlers.GetPlayerRadioHandler(store)(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetPlayerRadioHandlerUnknownSeed(t *testing.T) {
	store := newPlayerTestDB(t)
	req := httptest.NewRequest(http.MethodGet, "/apis/web/v1/player/radio?seed_track=999999", nil)
	rec := httptest.NewRecorder()

	handlers.GetPlayerRadioHandler(store)(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetPlayerDiscoverHandlerEmptyDB(t *testing.T) {
	store := newPlayerTestDB(t)
	req := httptest.NewRequest(http.MethodGet, "/apis/web/v1/player/discover?days=30&min_listens=5", nil)
	rec := httptest.NewRecorder()

	handlers.GetPlayerDiscoverHandler(store)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}
