package gamebox

import (
	"net/http"

	"github.com/gorilla/mux"
)

func registerAPIRoutes(r *mux.Router) {

	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}).Methods(http.MethodGet)

	r.HandleFunc("/tables", createTable()).Methods(http.MethodPost)
	r.HandleFunc("/tables", listTables()).Methods(http.MethodGet)
}
