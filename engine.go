package gamebox

import log "github.com/sirupsen/logrus"

// The Game Box Engine structure, container for turn based games
type Engine struct {
	name    string
	factory GameFactory

	// TODO: add internal state
}

// Factory to create the rules/logic object for the specific game
// Game implementation must provide a factory to create a new engine
type GameFactory func() GameRules

// Create a new engine based on a set of rules
func NewEngine(name string, f GameFactory) *Engine {
	log.Debugf("creating new engine for %s", name)
	return &Engine{
		name:    name,
		factory: f,
	}
}

// run the
func (e *Engine) Run(addr string) error {
	log.Infof("Gamebox for %s", e.name)
	// init the REST router
	// add REST handlers
	// add websocket handlers
	// start REST service
	log.Infof("Gamebox server listening on: %s", addr)
	return nil
}
