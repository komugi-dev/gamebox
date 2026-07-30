package main

import (
	"tic-tac-toe/logic"

	"github.com/komugi-dev/gamebox"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.DebugLevel)

	meta := gamebox.EngineMeta{
		Name:       "Tic-Tac-Toe",
		Version:    "1.0.0",
		Desc:       "Classic 3x3 grid game",
		MinPlayers: 2,
		MaxPlayers: 2,
	}

	log.Info("Starting Tic-Tac-Toe")

	e := gamebox.NewEngine(meta, logic.CreateT3)
	e.Run(8181)

}
