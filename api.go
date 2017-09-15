package main

import (
	"flag"
	"github.com/googollee/go-socket.io"
	"github.com/gorilla/mux"
	"github.com/moby/moby/client"
	"log"
	"net/http"
	"time"
)

var Port string
var Debug bool

func ParseFlags() {
	flag.StringVar(&Port, "port", "8080", "Listening on the given port")
	flag.BoolVar(&Debug, "debug", false, "whether to run in debug")
	flag.Parse()
}

//TODO:如果改用数据库而不是内存存储的话，考虑初始化的时候将数据从数据库加载进内存

func main() {
	ParseFlags()
	handler := &Handler{}
	c, err := client.NewEnvClient()
	if err != nil {
		log.Fatal(err)
	}
	handler.C = c
	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
		return
	}
	server.On("connection", handler.WS)
	server.On("error", handler.WSError)
	handler.So = server

	handler.S = make(map[string]*Session)
	handler.U = make(map[string]*User)

	r := mux.NewRouter()
	r.PathPrefix("/assets").Handler(http.FileServer(http.Dir("./")))
	r.HandleFunc("/experiment", handler.Index).Methods("GET")
	r.HandleFunc("/p/{sessionId}", handler.Home).Methods("GET")
	r.HandleFunc("/sessions/{sessionId}", handler.GetSession).Methods("GET")
	//r.HandleFunc("/sessions/{sessionId}/instances/{instanceId}/start",handler.ContainerStart).Methods("POST")
	//r.HandleFunc("/sessions/{sessionId}/instances/{instanceId}/stop",handler.StopContainer).Methods("POST")
	r.HandleFunc("/sessions/{sessionId}/instances/{instanceId}/delete", handler.ContainerRemove).Methods("DELETE")
	r.HandleFunc("/users/{username}/sessions/{sessionId}/store", handler.SessionStore).Methods("POST")
	r.HandleFunc("/users/{username}/sessions/{sessionId}/resume", handler.SessionResume).Methods("GET")
	r.HandleFunc("/users/{username}/sessions/{sessionId}/delete", handler.SessionDelete).Methods("POST")
	r.HandleFunc("/sessions/{sessionId}/instances/create", handler.ContainerCreate).Methods("POST")
	r.HandleFunc("/images/search", handler.ImageSearch).Methods("POST")
	r.Handle("/sessions/{sessionId}/ws/", server)
	httpServer := http.Server{
		Addr:              "0.0.0.0:" + Port,
		Handler:           r,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Println("Listening on : ", Port)
	httpServer.ListenAndServe()
}
