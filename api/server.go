package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/bducha/mbwol/grub"
	"github.com/bducha/mbwol/wol"
)

type BootRequest struct {
	ConfigName string `json:"configName"`
}

func ListenAndServe(port int) error {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /boot/{id}/{config}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		config := r.PathValue("config")
		slog.Debug("Received boot request", "id", id, "config", config)
		host, err := grub.GetHostById(id)
		if err != nil {
			http.Error(w, grub.ERR_HOST_NOT_FOUND, http.StatusNotFound)
			return
		}

		slog.Debug("Booting host", "host", host)
		broadcastIp := "255.255.255.255"
		if host.BroadcastIP != nil {
			broadcastIp = *host.BroadcastIP
		}
		err = wol.SendMagicPacket(host.MacAddress, broadcastIp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		slog.Debug("Setting host config", "config", config)
		err = grub.SetCurrentConfig(id, config)

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
	slog.Info(fmt.Sprintf("API listening on port %d", port))
	return http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}