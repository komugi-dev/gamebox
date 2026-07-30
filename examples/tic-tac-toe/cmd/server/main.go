package main

import (
	"tic-tac-toe/logic"

	"github.com/komugi-dev/gamebox"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.DebugLevel)
	log.Info("Starting Tic-Tac-Toe")

	e := gamebox.NewEngine("Tic-Tac-Toe", logic.CreateT3)
	e.Run(8181)

}
