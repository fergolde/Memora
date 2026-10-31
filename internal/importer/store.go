package importer

import "github.com/fergolde/memora/internal/db"

// importStore is the minimal interface required by all importer functions.
type importStore interface {
	db.ArtistStore
	db.AlbumStore
	db.TrackStore
	db.ListenStore
}
