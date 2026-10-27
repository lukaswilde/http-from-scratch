package http1

import (
	"bufio"
	"errors"
	"fmt"
	"io"
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

	response := NewResponse(conn)
	reader := bufio.NewReader(conn)

	for {
		request, err := ParseRequest(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			if e, ok := errors.AsType[HTTPError](err); ok {
				response.WriteHeader(e.StatusCode)
				response.header.Set("Connection", "close")
				err = response.finishRequest()
				return err
			}

			return fmt.Errorf("could not parse request: %w", err)
		}

		if request.Close {
			response.header.Set("Connection", "close")
		}
		mux.ServeHTTP(response, request)

		err = response.finishRequest()
		if err != nil {
			return fmt.Errorf("could not finish request: %w", err)
		}

		if request.Close {
			return nil
		}

		// TODO: set connection close in request
	}
}
