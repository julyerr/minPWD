package main

import (
	"sync"
	"github.com/satori/go.uuid"
	"log"
	"math"
)

type ViewPort struct {
	Rows uint
	Cols uint
}

type Client struct{
	Id       string
	ViewPort ViewPort
}

type Session struct{
	Id string `json:id`
	Instances map[string]*Instance `json:"instances"`
	Clients []*Client
	User *User `json:"user"`
	rw sync.Mutex
}

type SessionContent struct{
	Name string `json:"name"`
	Content string `json:"content"`
}

func (s *Session)Lock(){
	s.rw.Lock()
}

func (s *Session) Unlock(){
	s.rw.Unlock()
}

func (h *Handler)SessionNew(body *User) (*Session){
	s := &Session{}
	s.Instances = make(map[string]*Instance)
	if body ==nil || Debug{
		s.User.Name = "teacher1"
		s.User.IsTeacher = true
	}else{
		s.User = body
	}
	s.Id = uuid.NewV4().String()
	h.S[s.Id] = s
	h.U[s.User.Name] = &User{Name:s.User.Name,IsTeacher:s.User.IsTeacher}
	h.U[s.User.Name].Sessions = make(map[string][]string)
	return s
}


func (h *Handler)SessionClose(s *Session) error{
	s.Lock()
	defer  s.Unlock()
	h.So.BroadcastTo(s.Id,"session end")
	for _, i := range s.Instances {
		err := h.InstanceDelete(s, i)
		if err != nil {
			log.Println(err)
			return err
		}
	}
	delete(h.S,s.Id)
	return nil
}

func (s *Session) SessionGetSmallestViewPort() ViewPort {
	minRows := s.Clients[0].ViewPort.Rows
	minCols := s.Clients[0].ViewPort.Cols

	for _, c := range s.Clients {
		minRows = uint(math.Min(float64(minRows), float64(c.ViewPort.Rows)))
		minCols = uint(math.Min(float64(minCols), float64(c.ViewPort.Cols)))
	}

	return ViewPort{Rows: minRows, Cols: minCols}
}
