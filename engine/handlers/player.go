package handlers

import (
	"context"
	"net/http"

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
