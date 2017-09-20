package main

import (
	"flag"
	"log"
	"github.com/docker/docker/client"
	"github.com/docker/docker/api/types"
	"fmt"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/distribution/reference"
	"os"
	"context"
)

func ParseFlags(){
	flag.StringVar(&Port, "port", "8080", "Listening on the given port")
	flag.BoolVar(&Debug, "debug", false, "Whether to run in debug")
	flag.StringVar(&DBSchm,"dbschm","root:root@tcp(localhost:3306)/step1","The DB and database to connect")
	flag.StringVar(&defaultImage,"image","alpine","default image to launch if not set")
	flag.Parse()
}

func CheckPanic(err error) bool{
	if err != nil{
		log.Println(err)
		return true
	}
	return false
}

func CheckError(err error) bool{
	if err != nil{
		log.Println(err)
		return true
	}
	return false
}

func Boostrap() (*Handler,error) {
	ParseFlags()
	db,err := DBInit()
	if CheckPanic(err){
		return nil,err
	}
	handler := &Handler{}
	handler.DB = db

	c,err := client.NewEnvClient()
	if CheckPanic(err) {
		return nil,err
	}
	handler.DCL = c

	server,err := WSInit(handler)
	if err != nil {
		return nil,err
	}
	handler.SCK = server
	handler.Users=make(map[string]*User)
	err = UserFromDB(handler)
	if err != nil{
		return nil,err
	}
	return handler,nil
}

func CheckHostnameExists(s *Session, hostname string) bool {
	containerName := fmt.Sprintf("%s_%s", s.SessionId[:8], hostname)
	exists := false
	for _, instance := range s.Instances {
		if instance.InstanceName == containerName {
			exists = true
			break
		}
	}
	return exists
}

func PullImage(h *Handler,ctx context.Context, image string) error {
	_, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return err
	}

	options := types.ImageCreateOptions{}

	responseBody, err := h.DCL.ImageCreate(ctx, image, options)
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
