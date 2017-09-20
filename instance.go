package main

import (
	"net"
	"fmt"
	"context"
	"log"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/api/types/mount"
	"golang.org/x/text/encoding"
	"io"
	"strings"
)

type Instance struct{
	InstanceName string `json:"instance_name"`
	Config *InstanceConfig `json:"config"`
	Terminal net.Conn `json:"-"`
	Ip string `json:"ip"`
	//rw sync.Mutex
}

type InstanceConfig struct {
	ImageName string `json:"image_name"`
	Hostname string `json:"hostname"`
	IsMount bool `json:"is_mount"`
	MountSize int64 `json:"mount_size"`
	MemSize int64 `json:"mem_size"`
}
type AttachWriter struct {
	Handler  *Handler
	Instance *Instance
	Session  *Session
}

type ImageSearchConfig struct {
	Term     string `json:"term"`
	LimitNum int `json:"limit_num"`
}

func InstanceCreate(h *Handler,s *Session,config *InstanceConfig) (*Instance,error){
	s.Lock()
	defer s.Unlock()

	hostname := config.Hostname
	if hostname == "" {
		var nodeName string
		for i := 1; ; i++ {
			nodeName = fmt.Sprintf("node%d", i)
			exists := CheckHostnameExists(s, nodeName)
			if !exists {
				break
			}
		}
		hostname = nodeName
	}
	imageName := config.ImageName
	if imageName == ""{
		imageName = defaultImage
	}
	cf := &container.Config{
		Hostname:     hostname,
		Image:        imageName,
		Tty:          true,
		OpenStdin:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"sh"},
	}
	hc := &container.HostConfig{
		AutoRemove:true,
		LogConfig:  container.LogConfig{Config: map[string]string{"max-size": "10m", "max-file": "1"}},
	}
	mountSize := config.MountSize
	log.Println("is home dir mount:",config.IsMount)
	if config.IsMount {
		var mt mount.Mount
		if mountSize == 0 {
			mountSize = 4 * 1024 * 1024 * 1024
		}
		mt = mount.Mount{
			Type:   "bind",
			Source: "/home/" + s.User.UserName,
			Target: "/home/",
			//TmpfsOptions: &mount.TmpfsOptions{
			//	SizeBytes: mountSize,
			//},
		}
		hc.Mounts = []mount.Mount{mt}
	}
	memSize := config.MemSize
	if memSize == 0{
		memSize =  2048 * 1024 * 1024
	}
	hc.Resources.Memory = memSize
	t := true
	hc.Resources.OomKillDisable = &t

	containerName := fmt.Sprintf("%s_%s", s.SessionId[:8], hostname)
	fmt.Println("config container finished")
	container, err := h.DCL.ContainerCreate(context.Background(), cf, hc, nil, containerName)
	if err != nil {
		if client.IsErrImageNotFound(err) {
			log.Printf("Unable to find the image %s \n", imageName)
			if err = PullImage(h,context.Background(), imageName); CheckError(err) {
				return nil, err
			}
			container, err = h.DCL.ContainerCreate(context.Background(), cf, hc, nil, containerName)
			if CheckError(err) {
				return nil, err
			}
		}
	}
	fmt.Println("create container finished")
	err = h.DCL.ContainerStart(context.Background(), container.ID, types.ContainerStartOptions{})
	if CheckError(err){
		return nil, err
	}

	fmt.Println("start container finished")
	cinfo, err := h.DCL.ContainerInspect(context.Background(), container.ID)
	if CheckError(err) {
		return nil,err
	}
	instance := &Instance{InstanceName: containerName,Config:&InstanceConfig{
		Hostname:hostname,
		ImageName:imageName,
		IsMount:config.IsMount,
		MemSize:memSize,
	}, Ip: cinfo.NetworkSettings.DefaultNetworkSettings.IPAddress}
	if s.Instances == nil {
		s.Instances = make(map[string]*Instance)
	}
	s.Instances[instance.InstanceName] = instance
	go InstanceAttach(h,instance,s)
	h.SCK.BroadcastTo(s.SessionId, "new instance", instance.InstanceName, instance.Ip, instance.Config.Hostname)
	return instance,nil
}

func  InstanceAttach(h *Handler,i *Instance, s *Session) {
	conf := types.ContainerAttachOptions{true, true, true, true, "ctrl-^,ctrl-^", true}
	conn, err := h.DCL.ContainerAttach(context.Background(), i.InstanceName, conf)
	if CheckError(err){
		return
	}
	writer := &AttachWriter{Handler: h, Instance: i, Session: s}
	encoder := encoding.Replacement.NewEncoder()
	i.Terminal = conn.Conn
	io.Copy(encoder.Writer(writer), conn.Conn)
}

func (writer *AttachWriter) Write(p []byte) (n int, err error) {
	writer.Handler.SCK.BroadcastTo(writer.Session.SessionId, "terminal out", writer.Instance.InstanceName, string(p))
	return len(p), nil
}

func (is *Instance) WriteToTerminal(data string) {
	if is != nil && is.Terminal != nil && len(data) > 0 {
		is.Terminal.Write([]byte(data))
	}
}


func InstanceDelete(h *Handler,s *Session,i *Instance) error{
	s.Lock()
	defer s.Unlock()

	if i.Terminal != nil {
		i.Terminal.Close()
	}
	err := h.DCL.ContainerRemove(context.Background(), i.InstanceName, types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	if CheckError(err){
		//if instance exit automatic,just delete from the ui
		h.SCK.BroadcastTo(s.SessionId, "delete instance", i.InstanceName)
		delete(s.Instances, i.InstanceName)
		return err
	}
	h.SCK.BroadcastTo(s.SessionId, "delete instance", i.InstanceName)
	delete(s.Instances, i.InstanceName)
	return nil
}


func ImagesCommit(h *Handler,u *User,sessionId,content string) error{
	s := u.ActiveSessions[sessionId]
	s.Lock()
	s.Unlock()

	instances := make(map[string]*Instance)
	for instanceName,instance := range s.Instances{
		id, err := h.DCL.ContainerCommit(context.Background(),instanceName,types.ContainerCommitOptions{})
		if CheckError(err){
			return err
		}
		var imageId string
		if strings.Contains(id.ID, "sha256") {
			imageId = id.ID[7:17]
		} else {
			imageId = id.ID
		}
		instances[instanceName]=&Instance{
			InstanceName:instanceName,
			Config:&InstanceConfig{
				Hostname:instance.Config.Hostname,
				ImageName:imageId,
				IsMount:instance.Config.IsMount,
				MountSize:instance.Config.MountSize,
				MemSize:instance.Config.MemSize,
			},
		}
	}
	newSession := &Session{
		SessionId:sessionId,
		User:u,
		ImageName:s.ImageName,
		ExperimentName:s.ExperimentName,
		Instances:instances,
	}
	err := StoreSessionDB(h.DB,newSession,content)
	if err != nil{
		return err
	}
	if u.StoredSessions == nil{
		u.StoredSessions = make(map[string]*Session)
	}
	u.StoredSessions[sessionId]=newSession
	h.SCK.BroadcastTo(sessionId, "session stored", sessionId)
	return nil
}
func ImagesDelete(h *Handler,u *User,sessionId string)error{
	err := DeleteSessionDB(h.DB,u.StoredSessions[sessionId])
	if err != nil{
		return err
	}
	for _,v := range u.StoredSessions[sessionId].Instances {
		_, err := h.DCL.ImageRemove(context.Background(),v.Config.ImageName,types.ImageRemoveOptions{})
		if CheckError(err) {
			return err
		}
	}
	delete(u.StoredSessions,sessionId)
	if len(u.ActiveSessions) == 0 && len(u.ActiveSessions) == 0{
		delete(h.Users,u.UserName)
	}
	return nil
}

func ImageSearchOFF(h *Handler,term string,limit int) ([]string,error){
	if limit == 0{
		limit = 5
	}
	images, err := h.DCL.ImageSearch(context.Background(),term,
		types.ImageSearchOptions{Limit: limit})
	if CheckError(err){
		return nil,err
	}
	imageNames := []string{}
	for _, image := range images {
		imageNames = append(imageNames, image.Name)
	}
	return imageNames,nil
}

func NotifyClientSmallestViewPort(h *Handler,s *Session) {
	vp := SessionGetSmallestViewPoint(s)
	// Resize all terminals in the session
	h.SCK.BroadcastTo(s.SessionId, "viewport resize", vp.Cols, vp.Rows)
	for _, instance := range s.Instances {
		err := h.DCL.ContainerResize(context.Background(), instance.InstanceName, types.ResizeOptions{Height: vp.Rows, Width: vp.Cols})
		CheckError(err)
	}
}
