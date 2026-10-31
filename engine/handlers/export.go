package handlers

import (
	"net/http"

	"github.com/fergolde/memora/engine/middleware"
	"github.com/fergolde/memora/internal/db"
	"github.com/fergolde/memora/internal/export"
	"github.com/fergolde/memora/internal/logger"
	"github.com/fergolde/memora/internal/utils"
)

func ExportHandler(store db.ExportStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="koito_export.json"`)
		ctx := r.Context()
		l := logger.FromContext(ctx)
		l.Debug().Msg("ExportHandler: Received request for export file")
		u := middleware.GetUserFromContext(ctx)
		if u == nil {
			l.Debug().Msg("ExportHandler: Unauthorized access")
			utils.WriteError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		err := export.ExportData(ctx, u, store, w)
		if err != nil {
			l.Err(err).Msg("ExportHandler: Failed to create export file")
			utils.WriteError(w, "failed to create export file", http.StatusInternalServerError)
			return
		}
	}
}
