package httpserver

import (
	"context"
	"net/http"
	"time"
)

const (
	defaultAddr            = ":8080"
	defaultReadTimeout     = 5 * time.Second
	defaultWriteTimeout    = 5 * time.Second
	defaultIdleTimeout     = 60 * time.Second
	defaultShutdownTimeout = 3 * time.Second
)

type Server struct {
	httpServer *http.Server
	notify     chan error

	address         string
	readTimeout     time.Duration
	writeTimeout    time.Duration
	idleTimeout     time.Duration
	shutdownTimeout time.Duration
}

// New creates new http server with options.
func New(handler http.Handler, opts ...Option) *Server {
	s := &Server{
		address:         defaultAddr,
		readTimeout:     defaultReadTimeout,
		writeTimeout:    defaultWriteTimeout,
		idleTimeout:     defaultIdleTimeout,
		shutdownTimeout: defaultShutdownTimeout,
		notify:          make(chan error, 1),
	}

	// apply options
	for _, opt := range opts {
		opt(s)
	}

	s.httpServer = &http.Server{
		Addr:         s.address,
		Handler:      handler,
		ReadTimeout:  s.readTimeout,
		WriteTimeout: s.writeTimeout,
		IdleTimeout:  s.idleTimeout,
	}

	return s
}

func (s *Server) Start() {
	go func() {
		s.notify <- s.httpServer.ListenAndServe()
		close(s.notify)
	}()
}

func (s *Server) Notify() <-chan error {
	return s.notify
}

func (s *Server) Shutdown(ctx context.Context) error {
	// Only apply shutdownTimeout if caller didn't set a deadline.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.shutdownTimeout)
		defer cancel()
	}
	return s.httpServer.Shutdown(ctx)
}
