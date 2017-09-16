package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/docker/distribution/reference"
	"github.com/googollee/go-socket.io"
	"github.com/gorilla/mux"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"golang.org/x/text/encoding"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type Handler struct {
	C         *client.Client
	So        *socketio.Server
	S         map[string]*Session
	U         map[string]*User
	Instances map[string]*Instance
	Db        *sql.DB
}

type InstanceWriter struct {
	Handler  *Handler
	Instance *Instance
	Session  *Session
}

func (iw *InstanceWriter) Write(p []byte) (n int, err error) {
	iw.Handler.So.BroadcastTo(iw.Session.Id, "terminal out", iw.Instance.Name, string(p))
	return len(p), nil
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	s := h.SessionNew(r)
	http.Redirect(w, r, fmt.Sprintf("http://%s/p/%s", r.Host, s.Id), http.StatusFound)
}

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionId := vars["sessionId"]
	s := h.S[sessionId]
	if s == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, "index.html")
}

func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionId := vars["sessionId"]
	s := h.S[sessionId]
	if s == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(s)
}

func (h *Handler) ImageSearch(w http.ResponseWriter, r *http.Request) {
	body := &ImageSearchConfig{}
	json.NewDecoder(r.Body).Decode(&body)
	log.Printf("searching image %s, limit %d\n", body.Term, body.LimitNum)
	images, err := h.C.ImageSearch(context.Background(), body.Term,
		types.ImageSearchOptions{Limit: body.LimitNum})
	if err != nil {
		log.Println("image search failed")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	imageNames := []string{}
	for _, image := range images {
		imageNames = append(imageNames, image.Name)
	}
	json.NewEncoder(w).Encode(imageNames)
}

func (h *Handler) LocalImageSearch(w http.ResponseWriter, r *http.Request) {
	body := &ImageSearchConfig{}
	json.NewDecoder(r.Body).Decode(&body)
	log.Printf("searching image %s, limit %d\n", body.Term, body.LimitNum)
	images, err := h.ImageSearchDB(body.Term, body.LimitNum)
	if err != nil {
		log.Println("image search failed")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(images)
}

func (h *Handler) SessionStore(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	//对于用户输入的情况下进行条件判断 是否存在
	username := vars["username"]
	sessionId := vars["sessionId"]
	u, s := h.U[username], h.S[sessionId]
	if u == nil || s == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	body := &SessionContent{}
	json.NewDecoder(r.Body).Decode(&body)
	sessionNum := 0
	for k, _ := range u.Sessions {
		if k == sessionId {
			w.WriteHeader(http.StatusNotAcceptable)
			return
		}
		if sessionNum >= 1 {
			w.WriteHeader(http.StatusConflict)
			return
		}
		sessionNum += 1
	}

	images := make(map[string]string)
	for i, instance := range s.Instances {
		id, err := h.C.ContainerCommit(context.Background(), instance.Name, types.ContainerCommitOptions{})
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if strings.Contains(id.ID, "sha256") {
			images[i] = id.ID[7:17]
		} else {
			images[i] = id.ID
		}
	}
	eachSession := &EachSession{Instances:images,Resumed:false,Experiment:s.User.Sessions[sessionId].Experiment,
	ImageName:s.User.Sessions[sessionId].ImageName}
	u.Sessions[sessionId]=eachSession
	h.So.BroadcastTo(sessionId, "session stored", sessionId)
	err := h.StoreSessionDB(u, sessionId, body.Content)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (h *Handler) SessionResume(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]
	u := h.U[username]
	if u == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	sessionId := vars["sessionId"]
	if h.S[sessionId] != nil{
		w.Write([]byte("<a href=\"http://"+r.Host + "/p/" + sessionId+"\">click here to jump</a>"))
		return
	}
	s := &Session{}
	s.User = &User{}
	s.User = h.U[username]
	s.Instances = make(map[string]*Instance)
	s.Id = sessionId
	h.S[s.Id] = s
	if !u.Sessions[sessionId].Resumed {

		go func() {
			for k, v := range u.Sessions[sessionId].Instances {
				if _, err := h.containerCreate(s, k, v); err != nil {
					//w.WriteHeader(http.StatusInternalServerError)
					return
				}
				u.Sessions[sessionId].Resumed = true
			}
		}()
	} else {
		fmt.Printf("User %s session %s has been resumed\n", username, sessionId)
	}
	//waiting for client to get the session first
	//time.Sleep(time.Second * 1)
	w.Write([]byte("<a href=\"http://"+r.Host + "/p/" + s.Id+"\">click here to jump</a>"))
	//http.Redirect(w, r, fmt.Sprintf("http://%s/p/%s", r.Host, s.Id), http.StatusFound)
}

func (h *Handler) SessionDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]
	sessionId := vars["sessionId"]
	u, s := h.U[username], h.S[sessionId]
	if u == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	//该session正被使用
	if s != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	for _,v := range h.U[username].Sessions[sessionId].Instances {
		_, err := h.C.ImageRemove(context.Background(),v,types.ImageRemoveOptions{})
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	err := h.DeleteSessionDB(u, sessionId)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	delete(u.Sessions, sessionId)
	if len(u.Sessions) == 0 {
		delete(h.U, username)
	}
}

func (h *Handler) ContainerCreate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionId := vars["sessionId"]

	body := &InstanceConfig{}
	json.NewDecoder(r.Body).Decode(&body)
	s := h.S[sessionId]
	if s == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if len(s.Instances) >= 3 {
		w.WriteHeader(http.StatusConflict)
		return
	}
	instance, err := h.containerCreate(s, body.Hostname, body.ImageName)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	h.So.BroadcastTo(s.Id, "new instance", instance.Name, instance.Ip, instance.Hostname)
	json.NewEncoder(w).Encode(instance)
}

//err := h.C.ContainerStart(context.Background(), name, types.ContainerStartOptions{})
//err := h.C.ContainerStop(context.Background(), name, nil)

func (h *Handler) ContainerRemove(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionId := vars["sessionId"]
	instanceName := vars["instanceId"]
	//TODO:对于一些可能恶意攻击的请求，可能导致出错，后面加强判断和处理
	s := h.S[sessionId]
	instance := s.Instances[instanceName]
	if s == nil || instance == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if instance.Terminal != nil {
		instance.Terminal.Close()
	}
	err := h.C.ContainerRemove(context.Background(), instance.Name, types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	if err != nil {
		log.Println(err)
		delete(s.Instances, instanceName)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	h.So.BroadcastTo(sessionId, "delete instance", instanceName)
	delete(s.Instances, instanceName)
}

func (h *Handler) InstanceAttach(i *Instance, s *Session) {
	conf := types.ContainerAttachOptions{true, true, true, true, "ctrl-^,ctrl-^", true}
	conn, err := h.C.ContainerAttach(context.Background(), i.Name, conf)
	if err != nil {
		log.Println("container attach failed")
		return
	}
	iw := &InstanceWriter{Handler: h, Instance: i, Session: s}
	encoder := encoding.Replacement.NewEncoder()
	i.Terminal = conn.Conn
	io.Copy(encoder.Writer(iw), conn.Conn)
}

func (h *Handler) InstanceDelete(s *Session, i *Instance) error {
	if i.Terminal != nil {
		i.Terminal.Close()
	}
	err := h.C.ContainerRemove(context.Background(), i.Name, types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	if err != nil && !strings.Contains(err.Error(), "No such container") {
		log.Println(err)
		return err
	}
	h.So.BroadcastTo(s.Id, "delete instance", i.Name)
	delete(s.Instances, i.Name)
	return nil
}

func (h *Handler) notifyClientSmallestViewPort(session *Session) {
	vp := session.SessionGetSmallestViewPort()
	// Resize all terminals in the session
	h.So.BroadcastTo(session.Id, "viewport resize", vp.Cols, vp.Rows)
	for _, instance := range session.Instances {
		err := h.C.ContainerResize(context.Background(), instance.Name, types.ResizeOptions{Height: vp.Rows, Width: vp.Cols})
		if err != nil {
			log.Println("Error resizing terminal", err)
		}
	}
}

func (h *Handler) pullImage(ctx context.Context, image string) error {
	_, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return err
	}

	options := types.ImageCreateOptions{}

	responseBody, err := h.C.ImageCreate(ctx, image, options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	return jsonmessage.DisplayJSONMessagesStream(
		responseBody,
		os.Stderr,
		os.Stdout.Fd(),
		false,
		nil)
}

func (h *Handler) ExperimentContentGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	experiment := vars["experiment"]
	content, err := h.experimentContentGetDB(experiment)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write([]byte(content))
}

