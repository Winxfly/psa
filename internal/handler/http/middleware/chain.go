package middleware

import "net/http"

type Middleware func(http.Handler) http.Handler

type Chain struct {
	middlewares []Middleware
}

func NewChain() *Chain {
	return &Chain{
		middlewares: make([]Middleware, 0),
	}
}

func (c *Chain) Add(middleware ...Middleware) *Chain {
	c.middlewares = append(c.middlewares, middleware...)

	return c
}

func (c *Chain) Then(finalHandler http.Handler) http.Handler {
	if finalHandler == nil {
		panic("middleware: final handler is nil")
	}

	for i := len(c.middlewares) - 1; i >= 0; i-- {
		finalHandler = c.middlewares[i](finalHandler)
	}

	return finalHandler
}

func (c *Chain) ThenFunc(finalHandler http.HandlerFunc) http.Handler {
	return c.Then(finalHandler)
}
