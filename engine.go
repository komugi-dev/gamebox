package gamebox

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

// The Game Box Engine structure, container for turn based games
type Engine struct {
	Info     EngineMeta
	registry *gameRegistry
}

// Factory to create the rules/logic object for the specific game
// Game implementation must provide a factory to create a new registry
type GameFactory func() GameRules

// Create a new engine based on a set of rules
func NewEngine(meta EngineMeta, f GameFactory) *Engine {
	log.Debugf("creating new engine for %s", meta.Name)
	r := newGameRegistry(f)
	return &Engine{
		Info:     meta,
		registry: r,
	}
}

// run the engine
func (e *Engine) Run(port int) error {
	log.Infof("Gamebox for %+v", e.Info)
	var err error

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srvConf := serverConf{
		port:              port,
		readHeaderTimeout: 5 * time.Second,
		readTimeout:       15 * time.Second,
		writeTimeout:      35 * time.Second,
		idleTimeout:       60 * time.Second,
	}

	var srv *server
	if srv, err = newServer(srvConf, e.registry); err != nil {
		log.Fatalf("Server initialization failed: %v", err)
	}

	log.Infof("Gamebox server listening on: %v", srvConf.port)

	if err = srv.run(ctx); err != nil {
		log.Fatalf("Application error: %v", err)
	}

	return nil
}
