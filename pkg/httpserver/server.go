package httpserver

import (
	"context"
	"net/http"
	"time"
)

const (
	_defaultAddr            = ":8080"
	_defaultReadTimeout     = 5 * time.Second
	_defaultWriteTimeout    = 5 * time.Second
	_defaultShutdownTimeout = 3 * time.Second
)

type Server struct {
	httpServer *http.Server
	notify     chan error

	address         string
	readTimeout     time.Duration
	writeTimeout    time.Duration
	shutdownTimeout time.Duration
}

// New creates new http server with options.
func New(handler http.Handler, opts ...Option) *Server {
	s := &Server{
		address:         _defaultAddr,
		readTimeout:     _defaultReadTimeout,
		writeTimeout:    _defaultWriteTimeout,
		shutdownTimeout: _defaultShutdownTimeout,
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
	ctx, cancel := context.WithTimeout(ctx, s.shutdownTimeout)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}
