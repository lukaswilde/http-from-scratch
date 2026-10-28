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

		go func(c net.Conn) {
			defer c.Close()

			if err := HandleConnection(conn, handler); err != nil {
				log.Printf("[%s] connection error: %v", c.RemoteAddr(), err)
			}
		}(conn)
	}
}

func HandleConnection(conn net.Conn, mux http.Handler) error {
	log.Printf("Handling connection from %s", conn.RemoteAddr())

	reader := bufio.NewReader(conn)

	for {
		response := NewResponse(conn)
		request, err := ParseRequest(reader)
		// Could not read request line: client has closed the connection
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
			conn.Close()
			return nil
		}
	}
}
