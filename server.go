package gamebox

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

const (
	apiPrefix = "/gamebox/v1"
)

type serverConf struct {
	port              int
	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
}

type server struct {
	conf       serverConf
	httpServer *http.Server
}

func NewServer(c serverConf) (*server, error) {
	r := mux.NewRouter()

	api := r.PathPrefix(apiPrefix).Subrouter()

	registerAPIRoutes(api)

	return &server{
		conf: c,
		httpServer: &http.Server{
			Addr:              fmt.Sprintf(":%v", c.port),
			Handler:           r,
			ReadHeaderTimeout: c.readHeaderTimeout,
			ReadTimeout:       c.readTimeout,
			WriteTimeout:      c.writeTimeout,
			IdleTimeout:       c.idleTimeout,
		},
	}, nil
}

// Run starts the HTTP server and handles graceful shutdown
func (s *server) Run(ctx context.Context) error {
	serverErrors := make(chan error, 1)

	go func() {
		log.Infof("Server listening on port %s", s.httpServer.Addr)
		serverErrors <- s.httpServer.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if err != http.ErrServerClosed {
			return fmt.Errorf("server startup error: %w", err)
		}
	case <-ctx.Done():
		log.Info("Shutting down server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			_ = s.httpServer.Close()
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
	}

	log.Info("Server exited gracefully")
	return nil
}
