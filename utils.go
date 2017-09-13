package main

import "fmt"



type ImageSearchConfig struct {
	Term     string
	LimitNum int
}

func CheckHostnameExists(session *Session, hostname string) bool {
	containerName := fmt.Sprintf("%s_%s", session.Id[:8], hostname)
	exists := false
	for _, instance := range session.Instances {
		if instance.Name == containerName {
			exists = true
			break
		}
	}
	return exists
}
