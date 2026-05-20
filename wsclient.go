package gamebox

import "github.com/gorilla/websocket"

type client struct {
	conn       *websocket.Conn
	playerGUID string
	messages   chan ClientMsg
}

type ClientMsg struct {
}

func newClient(wsConn *websocket.Conn, playerGUID string) *client {
	return &client{
		conn:       wsConn,
		playerGUID: playerGUID,
		messages:   make(chan ClientMsg),
	}
}

func (c *client) writePump() {
	/*
		for msg := range c.send {
			c.conn.WriteMessage(websocket.TextMessage, msg)
		}
	*/
}

func (c *client) readPump() {}

func (c *client) run() {
	go c.writePump()
	c.readPump()
}

func (c *client) quit() {}
