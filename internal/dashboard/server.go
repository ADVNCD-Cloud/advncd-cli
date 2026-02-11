package dashboard

import (
	"net/http"
)

type Server struct {
	Addr    string
	Handler http.Handler
}

func NewServer() *Server {
	mux := http.NewServeMux()

	registerRoutes(mux)

	return &Server{
		Addr:    "127.0.0.1:7331",
		Handler: mux,
	}
}