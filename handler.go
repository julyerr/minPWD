package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/distribution/refrence"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/googollee/go-socket.io"
	"github.com/gorilla/mux"
	"golang.org/x/text/encoding"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Handler struct {
	C         *client.Client
	So        *socketio.Server
	S         map[string]*Session
	U         map[string]*User
	Instances map[string]*Instance
}

type InstanceWriter struct {
	Handler  *Handler
	Instance *Instance
	Session  *Session
}

func (iw *InstanceWriter) Write(p []byte) (n int, err error) {
	iw.Handler.So.BroadcastTo(iw.Session.Id, "terminal out", iw.Instance.Name, iw.Instance.Hostname, iw.Instance.Ip, string(p))
	return len(p), nil
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	body := &User{}
	json.NewDecoder(r.Body).Decode(&body)
	s := h.SessionNew(body)
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

func (h *Handler) SessionStore(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	//对于用户输入的情况下进行条件判断 是否存在
	username := vars["username"]
	sessionId := vars["sessionId"]
	body := &SessionContent{}
	json.NewDecoder(r.Body).Decode(&body)
	if len(h.U[username].Sessions) >= 1 {
		w.WriteHeader(http.StatusConflict)
		return
	}
	for k, _ := range h.U[username].Sessions {
		if k == sessionId {
			w.WriteHeader(http.StatusNotAcceptable)
			return
		}
	}
	images := []string{}
	for _, instance := range h.S[sessionId].Instances {
		id, err := h.C.ContainerCommit(context.Background(), instance.Name, types.ContainerCommitOptions{})
		if err != nil {
			log.Fatal(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		images = append(images, id[:10])
	}
	h.U[username].Sessions[sessionId] = images
	h.So.BroadcastTo(sessionId, "session stored", sessionId)
}

func (h *Handler) SessionResume(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]
	sessionId := vars["sessionId"]

	s := &Session{}
	s.User = &User{}
	s.Instances = make(map[string]*Instance)
	s.Id = sessionId
	h.S[s.Id] = s

	go http.Redirect(w, r, fmt.Sprintf("http://%s/p/%s", r.Host, s.Id), http.StatusFound)
	//waiting for client to get the session first
	time.Sleep(time.Second * 2)
	for _, v := range h.U[username].Sessions[sessionId] {
		if _, err := h.containerCreate(s, "", v); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func (h *Handler) SessionDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]
	sessionId := vars["sessionId"]
	//该session正被使用
	if h.S[sessionId] != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	delete(h.U[username].Sessions, sessionId)
	if len(h.U[username].Sessions) == 0 {
		delete(h.U, username)
	}
}

func (h *Handler) ContainerCreate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionId := vars["sessionId"]

	body := &InstanceConfig{}
	json.NewDecoder(r.Body).Decode(&body)
	s := h.S[sessionId]
	if len(s.Instances) >= 3 {
		w.WriteHeader(http.StatusConflict)
		return
	}
	instance, err := h.containerCreate(s, body.Hostname, body.ImageName)
	if err != nil {
		log.Fatal(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
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

	if instance.Terminal != nil {
		instance.Terminal.Close()
	}
	err := h.C.ContainerRemove(context.Background(), instance.Name, types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	if err != nil {
		log.Fatal(err)
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
		log.Fatal("container attach failed")
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
