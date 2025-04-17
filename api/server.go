package api

import (
	"log/slog"
	"net/http"

	"github.com/bducha/mbwol/grub"
)

type BootRequest struct {
	ConfigName string `json:"configName"`
}

func ListenAndServe() error {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /boot/{id}/{config}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		config := r.PathValue("config")

		err := grub.SetCurrentConfig(id, config)		

		if err != nil {
			switch (err.Error()) {
			case grub.ERR_CONFIG_NOT_FOUND:
				http.Error(w, grub.ERR_CONFIG_NOT_FOUND, http.StatusNotFound)
				return
			case grub.ERR_HOST_NOT_FOUND:
				http.Error(w, grub.ERR_HOST_NOT_FOUND, http.StatusNotFound)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})
	slog.Info("API listening on port 8000")
	return http.ListenAndServe(":8000", mux)
}