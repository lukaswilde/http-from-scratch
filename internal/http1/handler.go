package http1

import (
	"net/http"
	"strings"
)

var MyDefaultMux = &defaultMux

var defaultMux MyMux

type MyHandler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

type MyMux struct {
	routes map[string]func(http.ResponseWriter, *http.Request)
}

func (m MyMux) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	path := request.URL.Path

	for route, handler := range m.routes {
		if strings.HasSuffix(route, "/") {
			// match more than just the exact path
			_, found := strings.CutPrefix(path, route)
			if found {
				handler(writer, request)
				return
			}
		} else {
			if strings.EqualFold(route, path) {
				handler(writer, request)
				return
			}
		}
	}
	writer.WriteHeader(http.StatusNotFound)
}

func HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	if MyDefaultMux.routes == nil {
		MyDefaultMux.routes = make(map[string]func(http.ResponseWriter, *http.Request))
	}
	MyDefaultMux.routes[pattern] = handler
}
