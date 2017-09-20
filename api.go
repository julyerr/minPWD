package main

import (
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"
)

var Port string
var Debug bool
var DBSchm string
var defaultImage string


func main(){
	handler,err := Boostrap()
	if err != nil{
		return
	}

	r := mux.NewRouter()
	r.PathPrefix("/assets").Handler(http.FileServer(http.Dir("./")))
	r.HandleFunc("/experiment", handler.Index).Methods("GET")
	r.HandleFunc("/users/{username}/p/{sessionId}", handler.Home).Methods("GET")
	r.HandleFunc("/users/{username}/sessions/{sessionId}", handler.SessionGet).Methods("POST")
	r.HandleFunc("/users/{username}/sessions/{sessionId}/storedSessions", handler.StoredSessions).Methods("GET")
	r.HandleFunc("/users/{username}/sessions/{sessionId}/instances/{instanceId}/delete", handler.ContainerDelete).Methods("DELETE")
	r.HandleFunc("/users/{username}/sessions/{sessionId}/store", handler.SessionStore).Methods("POST")
	r.HandleFunc("/users/{username}/sessions/{sessionId}/resume", handler.SessionResume).Methods("GET")
	r.HandleFunc("/users/{username}/sessions/{sessionId}/delete", handler.SessionDelete).Methods("POST")
	r.HandleFunc("/users/{username}/sessions/{sessionId}/instances/create", handler.ContainerCreate).Methods("POST")
	r.HandleFunc("/images/search", handler.ImageSearch).Methods("POST")
	r.HandleFunc("/images/local/search", handler.LocalImageSearch).Methods("POST")
	r.HandleFunc("/experiment/{experimentName}", handler.ExperimentContentGet).Methods("POST")
	r.Handle("/users/{username}/sessions/{sessionId}/ws/", handler.SCK)
	httpServer := http.Server{
		Addr:              "0.0.0.0:" + Port,
		Handler:           r,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Println("Listening on : ", Port)
	httpServer.ListenAndServe()
}
