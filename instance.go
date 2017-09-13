package main

import (
	"net"
	"sync"
)

type Instance struct {
	Name     string   `json:name`
	Image    string   `json:image`
	Terminal net.Conn `json:-`
	Ip       string   `json:ip`
	Hostname string `json:"hostname"`
	rw sync.Mutex
}

type InstanceConfig struct {
	ImageName string
	IsMounted bool
	Hostname string
}

func (is *Instance) WriteToTerminal(data string) {
	if is != nil && is.Terminal != nil && len(data) > 0 {
		is.Terminal.Write([]byte(data))
	}
}
