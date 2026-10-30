package http1

import (
	"log"
	"net/http"
)

type MyHandler = http.Handler

type HandlerFunc func(http.ResponseWriter, *http.Request)

type MyMux struct {
	routingTree *RoutingTree[HandlerFunc]
}

func NewServeMux() *MyMux {
	return &MyMux{NewRoutingTree[HandlerFunc]()}
}

var MyDefaultMux = NewServeMux()

func (m *MyMux) HandleFunc(pattern string, handler HandlerFunc) {
	if err := m.routingTree.Insert(pattern, handler); err != nil {
		panic(err)
	}
}

func (m *MyMux) Handle(pattern string, handler MyHandler) {
	m.HandleFunc(pattern, handler.ServeHTTP)
}

func (m *MyMux) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	log.Println("Serving path ", path)

	handler, args, err := m.routingTree.Find(path)
	if err != nil {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	for k, v := range args {
		request.SetPathValue(k, v)
	}

	(*handler)(writer, request)
}

func HandleFunc(pattern string, handler HandlerFunc) {
	MyDefaultMux.HandleFunc(pattern, handler)
}
