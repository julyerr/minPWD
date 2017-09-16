package main

import (
	"sync"
	"github.com/satori/go.uuid"
	"log"
	"math"
	"fmt"
	"net/http"
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

func (h *Handler)SessionNew(r *http.Request) (*Session){
	s := &Session{}
	s.Instances = make(map[string]*Instance)
	s.Id = uuid.NewV4().String()
	s.User = &User{}
	s.User.Sessions=make(map[string]*EachSession)
	var imageName,experiment string
	r.ParseForm()
	if Debug && r.Form.Get("name") == "" {
		s.User.Name = "teacher1"
		s.User.IsTeacher = true
	}else{
		s.User.Name=r.Form.Get("name")
		imageName =r.Form.Get("image")
		experiment =r.Form.Get("experiment")
		isTeacher := r.Form.Get("isTeacher")
		if isTeacher == "teacher"{
			s.User.IsTeacher=true
		}else{
			s.User.IsTeacher=false
		}
	}
	h.S[s.Id] = s
	if _,exits := h.U[s.User.Name]; !exits{
		h.U[s.User.Name] = &User{Name:s.User.Name,IsTeacher:s.User.IsTeacher}
		h.U[s.User.Name].Sessions = make(map[string]*EachSession)
	}else{
		fmt.Printf("User %s info has been stored :\n",s.User.Name)
		for k,v := range h.U[s.User.Name].Sessions {
			eachSession := &EachSession{ImageName:v.ImageName,Experiment:v.Experiment,Instances:make(map[string]string),
			Resumed:v.Resumed}
			s.User.Sessions[k]=eachSession
			fmt.Printf("Session id %s :\n",k)
			for name,image := range v.Instances{
				fmt.Printf("name %s image %s:\n",name,image)
			}
		}
	}
	eachSession := &EachSession{ImageName:imageName,Experiment:experiment,Resumed:false}
	eachSession.Instances = make(map[string]string)
	s.User.Sessions[s.Id]=eachSession
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
	if u.Sessions[s.Id] != nil && u.Sessions[s.Id].Resumed {
		u.Sessions[s.Id].Resumed = false
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
