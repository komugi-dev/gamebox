package gamebox

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

// group the handlers
type API struct {
	registry *gameRegistry
}

type createTableRequest struct {
	TableName string `json:"table_name"`
}

type createTableResponse struct {
	TableGUID string `json:"table_guid"`
}

func (api *API) createTableHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// parse the request
		var req createTableRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		// invoke the service
		tableGUID := api.registry.createTable(req.TableName)
		resp := createTableResponse{TableGUID: tableGUID}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}

type listTableResponse struct {
	Tables []table
}

func (api *API) listTableHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tab := api.registry.listTables()
		resp := listTableResponse{
			Tables: tab,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

type joinTableRequest struct {
	PlayerName string `json:"player_name"`
}

type joinTableResponse struct {
	WebsocketSecret string `json:"websocket_secret"`
}

func (api *API) joinTableHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// get the table_guid
		vars := mux.Vars(r)
		tableGUID := vars["table_guid"]

		// get the player
		var req joinTableRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		// add a player
		wsSecret, err := api.registry.joinTable(tableGUID, req.PlayerName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// response
		resp := joinTableResponse{
			WebsocketSecret: wsSecret,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

type rejoinTableRequest struct {
	PlayerGUID string `json:"player_guid"`
}

type rejoinTableResponse struct {
	WebsocketSecret string `json:"websocket_secret"`
}

func (api *API) rejoinTableHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// get the table_guid
		vars := mux.Vars(r)
		tableGUID := vars["table_guid"]

		// get the player
		var req rejoinTableRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		// add a player
		wsSecret, err := api.registry.rejoinTable(tableGUID, req.PlayerGUID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// response
		resp := rejoinTableResponse{
			WebsocketSecret: wsSecret,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

type quitTableRequest struct {
	PlayerGUID string `json:"player_guid"`
}

func (api *API) quitTableHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// get the table_guid
		vars := mux.Vars(r)
		tableGUID := vars["table_guid"]

		// get the player
		var req quitTableRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		// quit
		err := api.registry.quitTable(tableGUID, req.PlayerGUID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// response
		w.WriteHeader(http.StatusNoContent)
	}
}

func (api *API) listPlayersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// get the table_guid
		vars := mux.Vars(r)
		tableGUID := vars["table_guid"]

		resp, err := api.registry.listPlayers(tableGUID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

func (api *API) startTableHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// get the table_guid
		vars := mux.Vars(r)
		tableGUID := vars["table_guid"]

		// start
		err := api.registry.startTable(tableGUID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// response
		w.WriteHeader(http.StatusNoContent)
	}
}

func (api *API) wsUpgradeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		secret := vars["secret"]
		guid, err := api.registry.joinConfirm(secret)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Debugf("ws for player %v", guid)

		// TODO: ws upgrade

	}
}
