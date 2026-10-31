package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/fergolde/memora/engine/middleware"
	"github.com/fergolde/memora/internal/cfg"
	"github.com/fergolde/memora/internal/db"
	"github.com/fergolde/memora/internal/logger"
	"github.com/fergolde/memora/internal/models"
	"github.com/fergolde/memora/internal/player"
	"github.com/fergolde/memora/internal/utils"
	"github.com/google/uuid"
)

// Player API (option B): read endpoints that enrich Memora items with the
// matching Subsonic song/album ID so external players can enqueue directly.
// subsonic_id is always nullable and a resolver miss never fails the request
// (match is one of mbid|metadata|none).

type PlayerTrack struct {
	*models.Track
	SubsonicID *string `json:"subsonic_id"`
	Match      string  `json:"match"`
}

type PlayerSimpleTrack struct {
	*models.SimpleTrack
	SubsonicID *string `json:"subsonic_id"`
	Match      string  `json:"match"`
}

type PlayerAlbum struct {
	*models.Album
	SubsonicID *string `json:"subsonic_id"`
	Match      string  `json:"match"`
}

type PlayerSearchResults struct {
	Tracks []PlayerTrack `json:"tracks"`
}

type PlayerResolveResult struct {
	Type       string  `json:"type"`
	SubsonicID *string `json:"subsonic_id"`
	Match      string  `json:"match"`
}

type playerTrackStore interface {
	db.TrackStore
}

type playerListenStore interface {
	db.ListenStore
}

type playerAlbumStore interface {
	db.AlbumStore
}

type playerRecommendStore interface {
	db.TrackStore
	db.ArtistStore
}

type playerRadioStore interface {
	db.TrackStore
	db.ArtistStore
}

type playerScrobbleStore interface {
	db.TrackStore
	db.ListenStore
}

type PlayerTrackList struct {
	Tracks []PlayerTrack `json:"tracks"`
}

type PlayerRadioQueue struct {
	Seed  PlayerTrack   `json:"seed"`
	Queue []PlayerTrack `json:"queue"`
}

func newPlayerResolver() *player.Resolver {
	return player.NewResolver(cfg.SubsonicUrl(), cfg.SubsonicParams(), http.DefaultClient)
}

func mbidString(mbid *uuid.UUID) *string {
	if mbid == nil {
		return nil
	}
	s := mbid.String()
	return &s
}

func firstArtistName(artists []models.SimpleArtist) string {
	if len(artists) == 0 {
		return ""
	}
	return artists[0].Name
}

func enrichTrack(ctx context.Context, res *player.Resolver, t *models.Track) PlayerTrack {
	l := logger.FromContext(ctx)
	id, match, err := res.ResolveSong(ctx, mbidString(t.MbzID), firstArtistName(t.Artists), t.Title)
	if err != nil {
		l.Err(err).Msg("enrichTrack: Subsonic resolution failed, returning unresolved item")
		return PlayerTrack{Track: t, Match: "none"}
	}
	return PlayerTrack{Track: t, SubsonicID: id, Match: match}
}

func enrichSimpleTrack(ctx context.Context, res *player.Resolver, t *models.SimpleTrack) PlayerSimpleTrack {
	l := logger.FromContext(ctx)
	id, match, err := res.ResolveSong(ctx, nil, firstArtistName(t.Artists), t.Title)
	if err != nil {
		l.Err(err).Msg("enrichSimpleTrack: Subsonic resolution failed, returning unresolved item")
		return PlayerSimpleTrack{SimpleTrack: t, Match: "none"}
	}
	return PlayerSimpleTrack{SimpleTrack: t, SubsonicID: id, Match: match}
}

func enrichAlbum(ctx context.Context, res *player.Resolver, a *models.Album) PlayerAlbum {
	l := logger.FromContext(ctx)
	id, match, err := res.ResolveAlbum(ctx, mbidString(a.MbzID), firstArtistName(a.Artists), a.Title)
	if err != nil {
		l.Err(err).Msg("enrichAlbum: Subsonic resolution failed, returning unresolved item")
		return PlayerAlbum{Album: a, Match: "none"}
	}
	return PlayerAlbum{Album: a, SubsonicID: id, Match: match}
}

func GetPlayerTopTracksHandler(store playerTrackStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromContext(ctx)
		opts := OptsFromRequest(r)

		tracks, err := store.GetTopTracksPaginated(ctx, opts)
		if err != nil {
			l.Err(err).Msg("GetPlayerTopTracksHandler: Failed to retrieve top tracks")
			utils.WriteError(w, "failed to get tracks", http.StatusBadRequest)
			return
		}

		res := newPlayerResolver()
		items := make([]db.RankedItem[PlayerTrack], 0, len(tracks.Items))
		for _, ranked := range tracks.Items {
			items = append(items, db.RankedItem[PlayerTrack]{
				Item: enrichTrack(ctx, res, ranked.Item),
				Rank: ranked.Rank,
			})
		}
		utils.WriteJSON(w, http.StatusOK, db.PaginatedResponse[db.RankedItem[PlayerTrack]]{
			Items:        items,
			TotalCount:   tracks.TotalCount,
			ItemsPerPage: tracks.ItemsPerPage,
			HasNextPage:  tracks.HasNextPage,
			CurrentPage:  tracks.CurrentPage,
		})
	}
}

func GetPlayerRecentHandler(store playerListenStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromContext(ctx)
		opts := OptsFromRequest(r)

		listens, err := store.GetListensPaginated(ctx, opts)
		if err != nil {
			l.Err(err).Msg("GetPlayerRecentHandler: Failed to retrieve listens")
			utils.WriteError(w, "failed to get listens", http.StatusBadRequest)
			return
		}

		res := newPlayerResolver()
		items := make([]PlayerSimpleTrack, 0, len(listens.Items))
		for _, li := range listens.Items {
			track := li.Track
			items = append(items, enrichSimpleTrack(ctx, res, &track))
		}
		utils.WriteJSON(w, http.StatusOK, db.PaginatedResponse[PlayerSimpleTrack]{
			Items:        items,
			TotalCount:   listens.TotalCount,
			ItemsPerPage: listens.ItemsPerPage,
			HasNextPage:  listens.HasNextPage,
			CurrentPage:  listens.CurrentPage,
		})
	}
}

func GetPlayerSearchHandler(store playerTrackStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromContext(ctx)
		q := r.URL.Query().Get("q")
		if q == "" {
			utils.WriteError(w, "query parameter 'q' is required", http.StatusBadRequest)
			return
		}

		tracks, err := store.SearchTracks(ctx, q)
		if err != nil {
			l.Err(err).Msg("GetPlayerSearchHandler: Failed to search tracks")
			utils.WriteError(w, "failed to search tracks", http.StatusBadRequest)
			return
		}

		res := newPlayerResolver()
		results := make([]PlayerTrack, 0, len(tracks))
		for _, t := range tracks {
			results = append(results, enrichTrack(ctx, res, t))
		}
		utils.WriteJSON(w, http.StatusOK, PlayerSearchResults{Tracks: results})
	}
}

func GetPlayerTopAlbumsHandler(store playerAlbumStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromContext(ctx)
		opts := OptsFromRequest(r)

		albums, err := store.GetTopAlbumsPaginated(ctx, opts)
		if err != nil {
			l.Err(err).Msg("GetPlayerTopAlbumsHandler: Failed to retrieve top albums")
			utils.WriteError(w, "failed to get albums", http.StatusBadRequest)
			return
		}

		res := newPlayerResolver()
		items := make([]db.RankedItem[PlayerAlbum], 0, len(albums.Items))
		for _, ranked := range albums.Items {
			items = append(items, db.RankedItem[PlayerAlbum]{
				Item: enrichAlbum(ctx, res, ranked.Item),
				Rank: ranked.Rank,
			})
		}
		utils.WriteJSON(w, http.StatusOK, db.PaginatedResponse[db.RankedItem[PlayerAlbum]]{
			Items:        items,
			TotalCount:   albums.TotalCount,
			ItemsPerPage: albums.ItemsPerPage,
			HasNextPage:  albums.HasNextPage,
			CurrentPage:  albums.CurrentPage,
		})
	}
}

func GetPlayerResolveHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		q := r.URL.Query()
		mbid := q.Get("mbid")
		artist := q.Get("artist")
		title := q.Get("title")
		resolveType := q.Get("type")
		if resolveType == "" {
			resolveType = "song"
		}
		if mbid == "" && (artist == "" || title == "") {
			utils.WriteError(w, "query parameter 'mbid' or 'artist'+'title' is required", http.StatusBadRequest)
			return
		}

		var mbidPtr *string
		if mbid != "" {
			mbidPtr = &mbid
		}
		res := newPlayerResolver()
		var id *string
		var match string
		var err error
		if resolveType == "album" {
			id, match, err = res.ResolveAlbum(ctx, mbidPtr, artist, title)
		} else {
			resolveType = "song"
			id, match, err = res.ResolveSong(ctx, mbidPtr, artist, title)
		}
		if err != nil {
			l := logger.FromContext(ctx)
			l.Err(err).Msg("GetPlayerResolveHandler: Subsonic resolution failed")
			utils.WriteError(w, "failed to resolve against Subsonic", http.StatusBadGateway)
			return
		}
		utils.WriteJSON(w, http.StatusOK, PlayerResolveResult{
			Type:       resolveType,
			SubsonicID: id,
			Match:      match,
		})
	}
}

func playerLimit(r *http.Request, def int) int {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 {
		return def
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func appendEnrichedTrack(ctx context.Context, res *player.Resolver, out []PlayerTrack, seen map[int32]bool, t *models.Track) []PlayerTrack {
	if t == nil || seen[t.ID] {
		return out
	}
	seen[t.ID] = true
	return append(out, enrichTrack(ctx, res, t))
}

func GetPlayerRecommendationsHandler(store playerRecommendStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromContext(ctx)
		limit := playerLimit(r, 25)
		seedArtist := r.URL.Query().Get("seed_artist")
		res := newPlayerResolver()
		seen := make(map[int32]bool)
		var tracks []PlayerTrack

		if seedArtist != "" {
			artists, err := store.SearchArtists(ctx, seedArtist)
			if err != nil {
				l.Err(err).Msg("GetPlayerRecommendationsHandler: Failed to search seed artist")
				utils.WriteError(w, "failed to search artist", http.StatusBadRequest)
				return
			}
			if len(artists) > 0 {
				byArtist, err := store.GetTracksByArtist(ctx, db.GetTracksByArtistOpts{ArtistID: artists[0].ID, Limit: limit})
				if err != nil {
					l.Err(err).Msg("GetPlayerRecommendationsHandler: Failed to get artist tracks")
					utils.WriteError(w, "failed to get tracks", http.StatusBadRequest)
					return
				}
				for _, t := range byArtist {
					tracks = appendEnrichedTrack(ctx, res, tracks, seen, t)
				}
			}
		}

		if len(tracks) < limit {
			top, err := store.GetTopTracksPaginated(ctx, db.GetItemsOpts{
				Limit: limit, Page: 1, Timeframe: db.Timeframe{Period: db.PeriodMonth},
			})
			if err != nil {
				l.Err(err).Msg("GetPlayerRecommendationsHandler: Failed to retrieve top tracks")
				utils.WriteError(w, "failed to get tracks", http.StatusBadRequest)
				return
			}
			for _, ranked := range top.Items {
				if len(tracks) >= limit {
					break
				}
				tracks = appendEnrichedTrack(ctx, res, tracks, seen, ranked.Item)
			}
		}
		utils.WriteJSON(w, http.StatusOK, PlayerTrackList{Tracks: tracks})
	}
}

func GetPlayerRadioHandler(store playerRadioStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromContext(ctx)
		limit := playerLimit(r, 25)

		seedID, err := strconv.Atoi(r.URL.Query().Get("seed_track"))
		if err != nil || seedID < 1 {
			l.Debug().Msg("GetPlayerRadioHandler: Invalid seed_track")
			utils.WriteError(w, "query parameter 'seed_track' (Memora track id) is required", http.StatusBadRequest)
			return
		}
		seed, err := store.GetTrack(ctx, db.GetTrackOpts{ID: int32(seedID)})
		if err != nil || seed == nil {
			l.Debug().Msg("GetPlayerRadioHandler: Seed track not found")
			utils.WriteError(w, "seed track not found", http.StatusNotFound)
			return
		}

		res := newPlayerResolver()
		seen := make(map[int32]bool)
		seedPlayer := enrichTrack(ctx, res, seed)
		seen[seed.ID] = true
		queue := []PlayerTrack{}

		artists, err := store.GetArtistsForTrack(ctx, seed.ID)
		if err != nil {
			l.Err(err).Msg("GetPlayerRadioHandler: Failed to get seed artists")
			utils.WriteError(w, "failed to build radio", http.StatusBadRequest)
			return
		}
		if len(artists) > 0 {
			peers, err := store.GetTracksByArtist(ctx, db.GetTracksByArtistOpts{ArtistID: artists[0].ID, Limit: limit})
			if err != nil {
				l.Err(err).Msg("GetPlayerRadioHandler: Failed to get artist tracks")
				utils.WriteError(w, "failed to build radio", http.StatusBadRequest)
				return
			}
			for _, t := range peers {
				if len(queue) >= limit-1 {
					break
				}
				before := len(queue)
				tmp := appendEnrichedTrack(ctx, res, queue, seen, t)
				if len(tmp) == before {
					continue
				}
				queue = tmp
			}
		}

		if len(queue) < limit-1 {
			top, err := store.GetTopTracksPaginated(ctx, db.GetItemsOpts{
				Limit: limit, Page: 1, Timeframe: db.Timeframe{Period: db.PeriodMonth},
			})
			if err != nil {
				l.Err(err).Msg("GetPlayerRadioHandler: Failed to retrieve top tracks")
				utils.WriteError(w, "failed to build radio", http.StatusBadRequest)
				return
			}
			for _, ranked := range top.Items {
				if len(queue) >= limit-1 {
					break
				}
				queue = appendEnrichedTrack(ctx, res, queue, seen, ranked.Item)
			}
		}
		utils.WriteJSON(w, http.StatusOK, PlayerRadioQueue{Seed: seedPlayer, Queue: queue})
	}
}

func GetPlayerDiscoverHandler(store playerTrackStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromContext(ctx)

		days, err := strconv.Atoi(r.URL.Query().Get("days"))
		if err != nil || days < 1 {
			days = 30
		}
		minListens, err := strconv.Atoi(r.URL.Query().Get("min_listens"))
		if err != nil || minListens < 1 {
			minListens = 5
		}

		tracks, err := store.GetForgottenFavorites(ctx, db.GetForgottenOpts{
			MinListens: minListens, Days: days, Limit: playerLimit(r, 25),
		})
		if err != nil {
			l.Err(err).Msg("GetPlayerDiscoverHandler: Failed to retrieve forgotten favorites")
			utils.WriteError(w, "failed to get tracks", http.StatusBadRequest)
			return
		}

		res := newPlayerResolver()
		seen := make(map[int32]bool)
		var out []PlayerTrack
		for _, t := range tracks {
			out = appendEnrichedTrack(ctx, res, out, seen, t)
		}
		utils.WriteJSON(w, http.StatusOK, PlayerTrackList{Tracks: out})
	}
}

var errMissingScrobbleID = errors.New("no track identifier provided")

// findScrobbleTrack maps a Subsonic song back to the Memora catalog, first by
// MBID and then by metadata search. Unknown tracks are left to the
// ListenBrainz endpoint, which auto-creates them.
func findScrobbleTrack(ctx context.Context, store playerScrobbleStore, song *player.Song) (*models.Track, error) {
	if song == nil {
		return nil, errMissingScrobbleID
	}
	if song.MusicBrainzID != "" {
		if mbid, err := uuid.Parse(song.MusicBrainzID); err == nil {
			if track, err := store.GetTrack(ctx, db.GetTrackOpts{MusicBrainzID: mbid}); err == nil && track != nil {
				return track, nil
			}
		}
	}
	tracks, err := store.SearchTracks(ctx, song.Artist+" "+song.Title)
	if err != nil || len(tracks) == 0 {
		return nil, errMissingScrobbleID
	}
	return tracks[0], nil
}

func PostPlayerScrobbleHandler(store playerScrobbleStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromContext(ctx)

		u := middleware.GetUserFromContext(ctx)
		if u == nil {
			l.Debug().Msg("PostPlayerScrobbleHandler: Unauthorized request (user context is nil)")
			utils.WriteError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		body, err := utils.DecodeBody[struct {
			TrackID    int32  `json:"track_id"`
			MBID       string `json:"mbid"`
			SubsonicID string `json:"subsonic_id"`
			Unix       int64  `json:"unix"`
			Client     string `json:"client"`
		}](r)
		if err != nil || body.Unix == 0 {
			l.Debug().Msg("PostPlayerScrobbleHandler: Invalid body")
			utils.WriteError(w, "track_id, mbid or subsonic_id plus unix timestamp must be provided", http.StatusBadRequest)
			return
		}
		if time.Now().Unix() < body.Unix {
			l.Debug().Msg("PostPlayerScrobbleHandler: Timestamp is in the future")
			utils.WriteError(w, "invalid timestamp", http.StatusBadRequest)
			return
		}

		var track *models.Track
		switch {
		case body.TrackID > 0:
			track, err = store.GetTrack(ctx, db.GetTrackOpts{ID: body.TrackID})
		case body.MBID != "":
			var mbid uuid.UUID
			if mbid, err = uuid.Parse(body.MBID); err == nil {
				track, err = store.GetTrack(ctx, db.GetTrackOpts{MusicBrainzID: mbid})
			}
		case body.SubsonicID != "":
			var song *player.Song
			if song, err = newPlayerResolver().GetSong(ctx, body.SubsonicID); err == nil {
				track, err = findScrobbleTrack(ctx, store, song)
			}
		default:
			err = errMissingScrobbleID
		}
		if err != nil || track == nil {
			l.Debug().Msg("PostPlayerScrobbleHandler: Track not in catalog")
			utils.WriteError(w, "track not in catalog, scrobble via the listenbrainz endpoint", http.StatusNotFound)
			return
		}

		client := body.Client
		if client == "" {
			client = "Memora Player"
		}
		if err := store.SaveListen(ctx, db.SaveListenOpts{
			TrackID: track.ID, Time: time.Unix(body.Unix, 0), UserID: u.ID, Client: client,
		}); err != nil {
			l.Err(err).Msg("PostPlayerScrobbleHandler: Failed to save listen")
			utils.WriteError(w, "failed to save listen", http.StatusInternalServerError)
			return
		}
		utils.WriteJSON(w, http.StatusCreated, map[string]int32{"track_id": track.ID})
	}
}
