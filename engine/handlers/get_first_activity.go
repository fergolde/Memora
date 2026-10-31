package handlers

import (
	"net/http"
	"time"

	"github.com/fergolde/memora/internal/db"
	"github.com/fergolde/memora/internal/logger"
	"github.com/fergolde/memora/internal/utils"
)

func FirstActivityHandler(store db.ListenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromContext(ctx)

		unix, err := store.GetFirstListenUnix(ctx)
		if err != nil {
			l.Debug().Err(err).Msg("Failed to get first listen unix")
			utils.WriteError(w, "failed to get first listen activity", 500)
		}

		utils.WriteJSON(w, 200, struct {
			Time time.Time `json:"time"`
		}{
			Time: time.Unix(unix, 0).UTC(),
		})
	}
}
