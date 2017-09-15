package main

import (
	"github.com/googollee/go-socket.io"
	"fmt"
	"log"
	"github.com/gorilla/mux"
)

func (h *Handler) WS(so socketio.Socket) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from ", r)
		}
	}()
	vars := mux.Vars(so.Request())

	sessionId := vars["sessionId"]
	s := h.S[sessionId]
	if s == nil {
		log.Printf("Session with id [%s] does not exist!\n", sessionId)
		return
	}
	so.Join(sessionId)
	c := &Client{Id:so.Id()}
	s.Clients = append(s.Clients,c)

	so.On("session close",func(username string){
		u := h.U[username]
		h.SessionClose(s,u)
	})
	so.On("terminal in", func(name,data string) {
		// User wrote something on the terminal. Need to write it to the instance terminal
		instance := h.S[sessionId].Instances[name]
		instance.WriteToTerminal(data)
	})
	//
	so.On("viewport resize", func(cols, rows uint) {
		// User resized his viewport
		c.ViewPort.Cols = cols
		c.ViewPort.Rows = rows
		h.notifyClientSmallestViewPort(s)
	})

	so.On("disconnection", func() {
		for i, cl := range s.Clients {
			if cl.Id == c.Id {
				s.Clients = append(s.Clients[:i], s.Clients[i+1:]...)
				break
			}
		}
		if len(s.Clients) > 0 {
			h.notifyClientSmallestViewPort(s)
		}else{
			if s1:=h.S[sessionId];s1 != nil {
				u := h.U[h.S[sessionId].User.Name]
				h.SessionClose(s,u)
			}
		}
	})
}

func (h *Handler) WSError(so socketio.Socket) {
	log.Println("error ws")
}
