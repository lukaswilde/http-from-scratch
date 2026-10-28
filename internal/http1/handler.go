package http1

import (
	"log"
	"net/http"
)

var MyDefaultMux = &defaultMux

var defaultMux = MyMux{NewRoutingTree[HandlerFunc]()}

type MyHandler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

type HandlerFunc func(http.ResponseWriter, *http.Request)

type MyMux struct {
	routingTree *RoutingTree[HandlerFunc]
}

func (m MyMux) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	log.Println("Serving path ", path)

	handler, err := m.routingTree.Find(path)
	if err != nil {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	(*handler)(writer, request)
}

func HandleFunc(pattern string, handler HandlerFunc) {
	err := MyDefaultMux.routingTree.Insert(pattern, handler)
	if err != nil {
		panic(err)
	}
}
