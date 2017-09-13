package main

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"log"
)

func (h *Handler) containerCreate(s *Session, hostname, imageName string) (*Instance, error) {
	s.Lock()
	defer s.Unlock()

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
		AutoRemove: true,
		LogConfig:  container.LogConfig{Config: map[string]string{"max-size": "10m", "max-file": "1"}},
	}
	hc.Resources.Memory = 2048 * 1024 * 1024
	t := true
	hc.Resources.OomKillDisable = &t

	containerName := fmt.Sprintf("%s_%s", s.Id[:8], hostname)
	container, err := h.C.ContainerCreate(context.Background(), cf, hc, nil, containerName)
	if err != nil {
		if client.IsErrImageNotFound(err) {
			log.Printf("Unable to find the image %s \n", imageName)
			if err = h.pullImage(context.Background(), imageName); err != nil {
				return nil, err
			}
			container, err = h.C.ContainerCreate(context.Background(), cf, h, nil, containerName)
			if err != nil {
				return nil, err
			}
		}
	}

	err = h.C.ContainerStart(context.Background(), container.ID, types.ContainerStartOptions{})
	if err != nil {
		return nil, err
	}

	cinfo, err := h.C.ContainerInspect(context.Background(), container.ID)
	if err != nil {
		log.Printf("Unable to inspect container %s \n", container.ID)
		return nil, err
	}

	instance := &Instance{Name: containerName,
		Image: imageName, Ip: cinfo.NetworkSettings.DefaultNetworkSettings.IPAddress}
	if s.Instances == nil {
		s.Instances = make(map[string]*Instance)
	}
	s.Instances[instance.Name] = instance
	h.So.BroadcastTo(s.Id, "new instance", instance.Name, instance.Ip, instance.Hostname)
	go h.InstanceAttach(instance, s)
	return instance, err
}
