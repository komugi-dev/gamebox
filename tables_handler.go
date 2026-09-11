package gamebox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const maxPlayerNameLen = 1024

// group the handlers
type API struct {
	registry *gameRegistry
}

func newAPI(registry *gameRegistry) *API {
	return &API{
		registry: registry,
	}
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

func (api *API) listTableHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := api.registry.listTables()

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

		if len(req.PlayerName) > maxPlayerNameLen {
			log.Warningf("cutting player name to %v", maxPlayerNameLen)
			req.PlayerName = req.PlayerName[:maxPlayerNameLen]
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
		err := api.registry.quitTable(player{tableGUID: tableGUID, guid: req.PlayerGUID})
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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (api *API) wsUpgradeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret := r.URL.Query().Get("secret")
		ticket, err := api.registry.consumeTicket(secret)
		if err != nil {
			log.Errorf("join confirm failed; %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Debugf("ws for ticket: %+v", ticket)

		// ws upgrade
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Errorf("web socket upgrade failed; %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Debug("ws connection upgrade OK")

		// run blocks until the web socket is alive
		client := newClient(conn, ticket.player.guid)
		tableInbox, playerOutbox, err := api.registry.seatPlayer(ticket.player, client)
		if err != nil {
			seatErr := fmt.Errorf("seat player failed; %v", err)
			log.Error(seatErr)
			closeMsg := websocket.FormatCloseMessage(websocket.CloseInternalServerErr, seatErr.Error())
			conn.WriteMessage(websocket.CloseMessage, closeMsg)
			conn.Close()
			return
		}
		log.Debugf("ws player on the table (client:%v)", client.playerGUID)

		welcome := msgPlayer{
			Type:       MsgTypeWelcome,
			PlayerGUID: ticket.player.guid,
		}
		if err := conn.WriteJSON(welcome); err != nil {
			log.Errorf("failed to send welcome message: %v", err)
			conn.Close()
			return
		}
		log.Debug("ws welcome message sent")

		err = client.run(context.Background(), tableInbox, playerOutbox)
		// the player was probably disconnected
		if err != nil {
			api.registry.disconnectPlayer(ticket.player.guid, ticket.tableGUID)
		}
	}
}
