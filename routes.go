package gamebox

import (
	"net/http"

	"github.com/gorilla/mux"
)

func registerAPIRoutes(r *mux.Router, api *API) {

	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}).Methods(http.MethodGet)

	r.HandleFunc("/tables", api.createTableHandler()).Methods(http.MethodPost)
	r.HandleFunc("/tables", api.listTableHandler()).Methods(http.MethodGet)
	r.HandleFunc("/tables/{table_guid}/join", api.joinTableHandler()).Methods(http.MethodPost)
	r.HandleFunc("/tables/{table_guid}/rejoin", api.rejoinTableHandler()).Methods(http.MethodPost)
	r.HandleFunc("/tables/{table_guid}/quit", api.quitTableHandler()).Methods(http.MethodPost)
	r.HandleFunc("/tables/{table_guid}/players", api.listPlayersHandler()).Methods(http.MethodGet)
	r.HandleFunc("/tables/{table_guid}/start", api.startTableHandler()).Methods(http.MethodPost)
	r.HandleFunc("/ws", api.wsUpgradeHandler()).Methods(http.MethodGet)
}
