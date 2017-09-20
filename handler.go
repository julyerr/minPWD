package main

import (
	"github.com/docker/docker/client"
	"github.com/googollee/go-socket.io"
	"database/sql"
	"net/http"
	"github.com/gorilla/mux"
	"encoding/json"
	"fmt"
	"log"
)

type Handler struct{
	DCL *client.Client
	SCK *socketio.Server
	DB *sql.DB
	Users map[string]*User
}

func (h *Handler) CheckSession(active bool ,r *http.Request) (bool,string,string){
	vars := mux.Vars(r)
	username := vars["username"]
	sessionId := vars["sessionId"]
	u := h.Users[username]
	if active{
		if u == nil || u.ActiveSessions == nil||u.ActiveSessions[sessionId]==nil{
			return false,"",""
		}
	}else{
		if u == nil || u.StoredSessions == nil||u.StoredSessions[sessionId]==nil{
			return false,"",""
		}
	}
	log.Println("session check username,sessionId,",username,sessionId)
	return true,username,sessionId
}

func (h *Handler)Index(w http.ResponseWriter,r *http.Request){
	s:= SessionNew("",r,"",h)
	if s == nil{
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("http://%s/users/%s/p/%s", r.Host,s.User.UserName, s.SessionId), http.StatusFound)
}

func (h *Handler)Home(w http.ResponseWriter,r *http.Request){
	exits,_,_ := h.CheckSession(true,r)
	if !exits{
		w.WriteHeader(http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, "index.html")
}

func (h *Handler)SessionGet(w http.ResponseWriter,r *http.Request){
	exits,username,sessionId := h.CheckSession(true,r)
	if !exits{
		w.WriteHeader(http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(h.Users[username].ActiveSessions[sessionId])
}

func (h *Handler)StoredSessions(w http.ResponseWriter,r *http.Request){
	exits,username,_ := h.CheckSession(true,r)
	if !exits{
		w.WriteHeader(http.StatusNotFound)
		return
	}
	storedSessions := []string{}
	for k,_ := range h.Users[username].StoredSessions{
		storedSessions=append(storedSessions,k)
	}
	json.NewEncoder(w).Encode(storedSessions)
}

func (h *Handler)ContainerCreate(w http.ResponseWriter,r *http.Request){
	exits,username,sessionId := h.CheckSession(true,r)
	if !exits{
		w.WriteHeader(http.StatusNotFound)
		return
	}
	body := &InstanceConfig{}
	json.NewDecoder(r.Body).Decode(&body)
	s := h.Users[username].ActiveSessions[sessionId]
	if len(s.Instances) >= 3 {
		w.WriteHeader(http.StatusConflict)
		return
	}
	i,err := InstanceCreate(h,s,body)
	if err !=nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(i)
}


func (h *Handler)ContainerDelete(w http.ResponseWriter,r *http.Request){
	exits,username,sessionId := h.CheckSession(true,r)
	vars := mux.Vars(r)
	if !exits {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	s := h.Users[username].ActiveSessions[sessionId]
	instanceId := vars["instanceId"]
	instance := s.Instances[instanceId]
	if instance == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	err := InstanceDelete(h,s,instance)
	if err != nil{
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (h *Handler)SessionStore(w http.ResponseWriter,r *http.Request){
	exits,username,sessionId := h.CheckSession(true,r)
	if !exits{
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	body := &SessionContent{}
	json.NewDecoder(r.Body).Decode(&body)
	u := h.Users[username]
	if len(u.ActiveSessions[sessionId].Instances) == 0 ||u.StoredSessions !=nil && (u.StoredSessions[sessionId] != nil||
	len(u.StoredSessions) >= 2){
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}
	err := ImagesCommit(h,u,sessionId,body.Content)
	if err != nil{
		w.WriteHeader(http.StatusInternalServerError)
	}
	return
}

func (h *Handler)SessionDelete(w http.ResponseWriter,r *http.Request){
	exits,username,sessionId := h.CheckSession(false,r)
	if !exits {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	log.Println("session delete,has such sesession? ",sessionId)
	s := h.Users[username].ActiveSessions[sessionId]
	if s != nil{
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}
	log.Println("session delete,the session being used? ",sessionId)
	err := ImagesDelete(h,h.Users[username],sessionId)
	if err != nil{
		w.WriteHeader(http.StatusInternalServerError)
	}
	return
}

func (h *Handler)SessionResume(w http.ResponseWriter,r *http.Request){
	exits,username,sessionId := h.CheckSession(false,r)
	if !exits {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	s := h.Users[username].ActiveSessions[sessionId]
	if s != nil{
		w.Write([]byte("<a href=\"http://"+r.Host + "/users/"+username+"/p/" + sessionId+"\">click here to jump</a>"))
		return
	}
	log.Println("session to resume id:",sessionId)
	if SessionNew(sessionId,nil,username,h) == nil{
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write([]byte("<a href=\"http://"+r.Host + "/users/"+username+"/p/" + sessionId+"\">click here to jump</a>"))
}

func (h *Handler) ImageSearch(w http.ResponseWriter,r *http.Request){
	body := &ImageSearchConfig{}
	json.NewDecoder(r.Body).Decode(&body)
	images,err := ImageSearchOFF(h,body.Term,body.LimitNum)
	if err != nil{
		w.WriteHeader(http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(images)
}

func (h *Handler) LocalImageSearch(w http.ResponseWriter,r *http.Request){
	body := &ImageSearchConfig{}
	json.NewDecoder(r.Body).Decode(&body)
	log.Println("image Searching term,limitNum,",body.Term,body.LimitNum)
	images,err := ImageSearchDB(h.DB,body.Term,body.LimitNum)
	if err != nil{
		w.WriteHeader(http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(images)
}

func (h *Handler) ExperimentContentGet(w http.ResponseWriter,r *http.Request){
	vars := mux.Vars(r)
	experiment := vars["experimentName"]
	content, err := ContentGetDB(h.DB,experiment)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write([]byte(content))
}

