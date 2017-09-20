package main

import (
	"github.com/googollee/go-socket.io"
	"fmt"
	"log"
)

func WSInit(h *Handler) (*socketio.Server,error){
	server, err := socketio.NewServer(nil)
	if CheckPanic(err){
		return nil,err
	}
	server.On("connection", h.WS)
	server.On("error", h.WSError)
	return server,nil
}

func (h *Handler)WS(so socketio.Socket) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from ", r)
		}
	}()
	exits,username,sessionId := h.CheckSession(true,so.Request())
	if !exits{
		return
	}
	so.Join(sessionId)
	c := &Client{Id:so.Id()}
	u := h.Users[username]
	s := u.ActiveSessions[sessionId]
	s.Clients = append(s.Clients,c)

	so.On("session close",func(username string){
		SessionClose(h,u,s)
	})
	so.On("terminal in", func(name,data string) {
		// User wrote something on the terminal. Need to write it to the instance terminal
		instance := s.Instances[name]
		instance.WriteToTerminal(data)
	})
	//
	so.On("viewport resize", func(cols, rows uint) {
		// User resized his viewport
		c.ViewPoint.Cols = cols
		c.ViewPoint.Rows = rows
		NotifyClientSmallestViewPort(h,s)
	})

	so.On("disconnection", func() {
		for i, cl := range s.Clients {
			if cl.Id == c.Id {
				s.Clients = append(s.Clients[:i], s.Clients[i+1:]...)
				break
			}
		}
		if len(s.Clients) > 0 {
			NotifyClientSmallestViewPort(h,s)
		}else{
			if s != nil {
				SessionClose(h,u,s)
				log.Printf("no client connected,close session %s automaticly",sessionId)
			}
		}
	})
}

func  (h *Handler)WSError(so socketio.Socket) {
	log.Println("error ws")
}

