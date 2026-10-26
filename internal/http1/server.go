package http1

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
)

func ListenAndServe(addr string, handler http.Handler) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()

	if handler == nil {
		handler = http.DefaultServeMux
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("could not accept request: %w", err)
		}

		if err = HandleConnection(conn, handler); err != nil {
			return fmt.Errorf("could not handle connection: %w", err)
		}
	}
}

func HandleConnection(conn net.Conn, mux http.Handler) error {
	log.Printf("Handling connection from %s", conn.RemoteAddr())
	defer conn.Close()

	reader := bufio.NewReader(conn)
	request, err := ParseRequest(reader)
	if err != nil {
		return fmt.Errorf("could not parse request: %w", err)
	}

	response := NewResponse(conn)
	mux.ServeHTTP(response, request)

	return nil
}
