package main 

import (
	"github.com/gorilla/mux"
	"net/http"
	"flag"
	"time"
	"github.com/docker/docker/client"
	"github.com/docker/docker/api/types"
	"log"
	"context"
	//"encoding/json"
	"github.com/googollee/go-socket.io"
	"fmt"
	"golang.org/x/text/encoding"
	"net"
	"io"
	"encoding/json"
	"github.com/docker/docker/api/types/container"
	//"golang.org/x/text/message"
)

var Port string
var IS *Instance

func main(){
	ParseFlags()
	IS = &Instance{}
	c,err := client.NewEnvClient()
	if err != nil{
		log.Fatal(err)
	}
	IS.c = c
	server ,err := socketio.NewServer(nil)
	if err != nil{
		log.Fatal(err)
		return
	}
	server.On("connection", IS.WS)
	server.On("error", IS.WSError)
	IS.so = server

	r := mux.NewRouter()
	r.PathPrefix("/assets").Handler(http.FileServer(http.Dir("./")))
	r.HandleFunc("/{containerId}",IS.Index).Methods("GET")
	r.HandleFunc("/{containerId}/status",IS.ContainerStatus).Methods("GET")
	r.HandleFunc("/{containerId}/start",IS.StartContainer).Methods("GET")
	r.HandleFunc("/{containerId}/stop",IS.StopContainer).Methods("GET")
	r.HandleFunc("/{containerId}/remove",IS.RemoveContainer).Methods("GET")
	r.HandleFunc("/{containerId}/store",IS.StoreContainer).Methods("GET")
	r.HandleFunc("/{imageId}/resume",IS.ResumeContainer).Methods("GET")
	r.Handle("/{containerId}/ws/",server)
	httpServer := http.Server{
		Addr:"0.0.0.0:"+Port,
		Handler:r,
		IdleTimeout:30*time.Second,
		ReadHeaderTimeout:5*time.Second,
	}
	httpServer.ListenAndServe()
}

func ParseFlags(){
	flag.StringVar(&Port,"port","8080","Listening on the given port")
	flag.Parse()
}

type containerStatus struct{
	Stopped bool `json:"stopped"`
	StoreInfo []string `json:"store_info"`
}

func (is *Instance) Index(w http.ResponseWriter,r *http.Request){
	http.ServeFile(w,r,"index.html")
}

func (is *Instance) ContainerStatus(w http.ResponseWriter,r *http.Request){
	containerId := mux.Vars(r)["containerId"]
	is.name = containerId
	info ,err := is.c.ContainerInspect(context.Background(),is.name)
	if err != nil{
		log.Fatal(err)
		w.Write([]byte("get container info failed"))
		return
	}
	result := &containerStatus{}
	result.Stopped = info.State.Paused
	result.StoreInfo = []string{"test1","test2"}
	json.NewEncoder(w).Encode(result)
}

func (is *Instance) StartContainer(w http.ResponseWriter,r *http.Request){
	containerId := mux.Vars(r)["containerId"]
	err := is.c.ContainerStart(context.Background(),containerId , types.ContainerStartOptions{})
	if err != nil {
		log.Fatal(err)
		w.Write([]byte("starting container failed"))
		return
	}
}

func (is *Instance) StopContainer(w http.ResponseWriter,r *http.Request){
	containerId := mux.Vars(r)["containerId"]
	dur := time.Duration(1)*time.Second
	err := is.c.ContainerStop(context.Background(),containerId,&dur)
	if err != nil {
		log.Fatal(err)
		w.Write([]byte("stop container failed"))
		return
	}
}
func (is *Instance) RemoveContainer(w http.ResponseWriter,r *http.Request){
	containerId := mux.Vars(r)["containerId"]
	err := is.c.ContainerRemove(context.Background(), containerId, types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	if err != nil{
		log.Fatal(err)
		w.Write([]byte("remove container failed"))
		return
	}
}
func (is *Instance) StoreContainer(w http.ResponseWriter,r *http.Request){
	containerId := mux.Vars(r)["containerId"]
	id,err := is.c.ContainerCommit(context.Background(),containerId,types.ContainerCommitOptions{})
	if err != nil{
		log.Fatal(err)
		w.Write([]byte("commit container failed"))
		return
	}
	json.NewEncoder(w).Encode(id)
	fmt.Printf("container commit id :%s" ,id.ID)
}

func  (is *Instance) ResumeContainer(w http.ResponseWriter,r *http.Request){
	imageId := mux.Vars(r)["imageId"]
	cf := &container.Config{
		Image:        imageId,
		Tty:          true,
		OpenStdin:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:[]string{"sh"},
	}
	h := &container.HostConfig{
		AutoRemove:  false,
		LogConfig:   container.LogConfig{Config: map[string]string{"max-size": "10m", "max-file": "1"}},
	}
	h.Resources.Memory = 2048 * 1024 * 1024
	container,err := is.c.ContainerCreate(context.Background(),cf,h,nil,"")
	if err != nil{
		if client.IsErrImageNotFound(err){
			log.Printf("Unable to find the image %s \n",imageId)
			w.Write([]byte("Unable to find the image"))
			return
		}
	}
	err = is.c.ContainerStart(context.Background(),container.ID,types.ContainerStartOptions{})
	if err != nil{
		log.Printf("Unable to start container %s \n",container.ID)
		w.Write([]byte("Unable to start the container"))
		return
	}
	w.Write([]byte(container.ID))
}

func (is *Instance)WS(so socketio.Socket) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from ", r)
		}
	}()
	vars := mux.Vars(so.Request())

	containerId := vars["containerId"]
	is.name = containerId
	so.Join(containerId)
	ctx := context.Background()

	conf := types.ContainerAttachOptions{true, true, true, true, "ctrl-^,ctrl-^", true}
	conn, err := is.c.ContainerAttach(ctx,containerId, conf)
	if err != nil {
		log.Fatal("container attach failed")
		return
	}
	encoder := encoding.Replacement.NewEncoder()
	is.Terminal = conn.Conn
	go io.Copy(encoder.Writer(is),conn.Conn)
	so.On("terminal in", func(data string) {
		fmt.Print("terminal in , data : ",data)
		// User wrote something on the terminal. Need to write it to the instance terminal
		is.WriteToTerminal(data)
	})
	//
	so.On("viewport resize", func(cols, rows uint) {
		// User resized his viewport
		fmt.Println("viewport resize, cols,rows int ",cols,rows)
		is.c.ContainerResize(context.Background(),containerId, types.ResizeOptions{Height: rows, Width: cols})
		is.so.BroadcastTo(is.name,"viewport resize",cols, rows)
	})
	//
	so.On("connect",func(){
		fmt.Println("connected")
	})
	so.On("disconnection", func() {
		fmt.Println("client disconnection")
	})
}

type Instance struct{
	name string
	Terminal net.Conn
	so *socketio.Server
	c *client.Client
}

func (is *Instance) Write(p []byte) (n int, err error){
	is.so.BroadcastTo(is.name,"terminal out",string(p))
	return len(p),nil
}

func (is *Instance)WriteToTerminal(data string){
	if is != nil && is.Terminal != nil && len(data) > 0{
		is.Terminal.Write([]byte(data))
	}
}

func (is *Instance)WSError(so socketio.Socket) {
	log.Println("error ws")
}





