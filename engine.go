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
	Name     string
	registry *gameRegistry

	// TODO: add internal state
}

// Factory to create the rules/logic object for the specific game
// Game implementation must provide a factory to create a new registry
type GameFactory func() GameRules

// Create a new engine based on a set of rules
func NewEngine(name string, f GameFactory) *Engine {
	log.Debugf("creating new engine for %s", name)
	r := newGameRegistry(f)
	return &Engine{
		Name:     name,
		registry: r,
	}
}

// run the engine
func (e *Engine) Run(port int) error {
	log.Infof("Gamebox for %s", e.Name)

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
