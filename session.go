package main

import (
	"sync"
	"github.com/satori/go.uuid"
	"log"
	"math"
	"fmt"
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
	Id string `json:"id"`
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
	s.User = &User{}
	if body ==nil || Debug{
		s.User.Name = "teacher1"
		s.User.IsTeacher = true
	}else{
		s.User = body
	}
	s.Id = uuid.NewV4().String()
	h.S[s.Id] = s
	if _,exits := h.U[s.User.Name]; !exits{
		h.U[s.User.Name] = &User{Name:s.User.Name,IsTeacher:s.User.IsTeacher}
		h.U[s.User.Name].Sessions = make(map[string]map[string]string)
		h.U[s.User.Name].Resumes = make(map[string]bool)
	}else{
		s.User = h.U[s.User.Name]
		fmt.Printf("User %s info has been stored :\n",s.User.Name)
		for k,v := range h.U[s.User.Name].Sessions {
			fmt.Printf("Session id %s :\n",k)
			for name,image := range v{
				fmt.Printf("name %s image %s:\n",name,image)
			}
		}
	}
	return s
}


func (h *Handler)SessionClose(s *Session,u *User) error{
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
	if u.Resumes[s.Id] {
		u.Resumes[s.Id] = false
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
