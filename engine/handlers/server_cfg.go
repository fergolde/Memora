package handlers

import (
	"net/http"

	"github.com/fergolde/memora/internal/cfg"
	"github.com/fergolde/memora/internal/utils"
)

type ServerConfig struct {
	DefaultTheme string `json:"default_theme"`
}

func GetCfgHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		utils.WriteJSON(w, http.StatusOK, ServerConfig{DefaultTheme: cfg.DefaultTheme()})
	}
}
