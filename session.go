package main

import (
	"sync"
	"net/http"
	"github.com/satori/go.uuid"
	"math"
	"strings"
	"log"
)

type ViewPoint struct{
	Rows uint
	Cols uint
}

type Client struct {
	Id string
	ViewPoint ViewPoint
}

type Session struct {
	SessionId string `json:"session_id"`
	User *User `json:"user"`
	ImageName string `json:"image_name"`
	ExperimentName string `json:"experiment_name"`
	Instances map[string]*Instance `json:"instances"`

	Clients []*Client `json:"-"`
	rw sync.Mutex
}
type SessionContent struct{
	Name string `json:"name"`
	Content string `json:"content"`
}

func (s *Session) Lock(){
	s.rw.Lock()
}

func (s *Session) Unlock(){
	s.rw.Unlock()
}

func SessionNew(id string,r *http.Request,username string,h *Handler) (*Session){
	s := &Session{}
	log.Printf("id %s\n",id)
	log.Println("session id same ?",strings.EqualFold(id,""))
	if id == ""{
		var name string
		var isTeacher bool
		r.ParseForm()
		if Debug && r.Form.Get("name") == "" {
			name = "teacher1"
			isTeacher = true
		}else {
			name = r.Form.Get("name")
			isteacher := r.Form.Get("isTeacher")
			if isteacher == "teacher"{
				isTeacher=true
			}else{
				isTeacher=false
			}
		}
		s.SessionId = uuid.NewV4().String()
		s.ImageName = r.Form.Get("image")
		s.ExperimentName = r.Form.Get("experiment")
		if h.Users[name] == nil {
			h.Users[name] = &User{UserName:name,IsTeacher:isTeacher}
		}
		if h.Users[name].ActiveSessions == nil{
			h.Users[name].ActiveSessions = make(map[string]*Session)
		}
		if len(h.Users[name].ActiveSessions) >= 5{
			return nil
		}
		h.Users[name].ActiveSessions[s.SessionId]=s
		s.User = h.Users[name]
	}else{
		u := h.Users[username]
		u.Lock()
		defer u.Unlock()

		s.SessionId = id
		s.ImageName = u.StoredSessions[id].ImageName
		s.ExperimentName = u.StoredSessions[id].ExperimentName
		s.User = u
		if u.ActiveSessions == nil{
			u.ActiveSessions = make(map[string]*Session)
		}
		instances := u.StoredSessions[id].Instances
		for _, v := range instances {
			_,err := InstanceCreate(h,s,v.Config)
			if CheckError(err){
				return nil
			}
		}
		u.ActiveSessions[id]=s
	}
	return s
}


func SessionClose(h *Handler,u *User,s *Session) error{
	u.Lock()
	defer u.Unlock()

	h.SCK.BroadcastTo(s.SessionId,"session end")
	for _, i := range s.Instances {
		err := InstanceDelete(h,s, i)
		if err != nil {
			return err
		}
	}
	delete(u.ActiveSessions,s.SessionId)
	if len(u.ActiveSessions) == 0 && len(u.StoredSessions) == 0{
		delete(h.Users,u.UserName)
	}
	return nil
}


func SessionGetSmallestViewPoint(s *Session) ViewPoint {
	minRows := s.Clients[0].ViewPoint.Rows
	minCols := s.Clients[0].ViewPoint.Cols

	for _, c := range s.Clients {
		minRows = uint(math.Min(float64(minRows), float64(c.ViewPoint.Rows)))
		minCols = uint(math.Min(float64(minCols), float64(c.ViewPoint.Cols)))
	}

	return ViewPoint{Rows: minRows, Cols: minCols}
}


