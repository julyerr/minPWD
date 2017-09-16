package main

import (
	"log"
)

//sessionId = db.Column(db.String(60),nullable = False,primary_key=True)
//sessionComment = db.Column(db.String(200))
//name = db.Column(db.String(20),nullable=False)
//isTeacher = db.Column(db.Boolean)
//experiment = db.Column(db.String(40))

func (h *Handler) StoreSessionDB(u *User,sessionId ,content string) error{
	_,err := h.Db.Exec("INSERT INTO `sessions`(`sessionId`,`sessionComment`,`name`,`experiment`,`isTeacher`,`image`) VALUES "+
	"(",sessionId,content,u.Name,u.Sessions[sessionId].Experiment,u.IsTeacher,u.Sessions[sessionId].ImageName,")")
	if err != nil{
		log.Println("insert sessions error",err.Error())
		return err
	}
	for k,v := range u.Sessions[sessionId].Instances{
		_,err =h.Db.Exec("INSERT INTO `images` (`imageId`,`hostname`,`sessionId`) VALUES"+
		"(",v,k,sessionId,")")
		if err != nil{
			log.Println("insert sessions error",err.Error())
			return err
		}
	}
	return nil
}

func (h *Handler) DeleteSessionDB(u *User,sessionId string) error{
	_,err := h.Db.Exec("DELETE FROM `images` WHERE `sessionId`=?",sessionId)
	if err != nil{
		log.Println("delete images from session failed ",err)
		return err
	}
	_,err = h.Db.Exec("DELETE FROM `sessions` WHERE `sessionId`=?",sessionId)
	if err != nil{
		log.Println("delete images from session failed ",err)
		return err
	}
	return nil
}

func (h *Handler) UserFromDB(){
	h.U = make(map[string]*User)
	rows,err := h.Db.Query("SELECT * FROM `sessions`")
	if err != nil{
		log.Println("fetch user info from db failed ",err.Error())
		return
	}
	defer rows.Close()
	for rows.Next(){
		var sessionId,sessionComment,name,image,experiment string
		var isTeacher bool
		rows.Scan(&sessionId,&sessionComment,&name,&image,&experiment,&isTeacher)
		log.Println("fetch user info :",name)
		u := h.U[name]
		if u== nil{
			u = &User{Name:name,IsTeacher:isTeacher}
			u.Sessions=make(map[string]*EachSession)
			h.U[name]=u
		}
		images := make(map[string]string)

		rows1,err := h.Db.Query("SELECT * from `images`")
		if err != nil{
			log.Printf("fetch image info from session %s failed ",sessionId,err.Error())
			return
		}
		defer rows1.Close()

		for rows1.Next(){
			var imageId,hostname string
			rows1.Scan(&imageId,&hostname)
			images[hostname]=imageId
			log.Println("load images from session %s ",sessionId)
		}
		eachSession := &EachSession{Experiment:experiment,Resumed:false,ImageName:image,Instances:images}
		h.U[name].Sessions[sessionId]=eachSession
	}
}

func (h *Handler) ImageSearchDB(term string,limit int) ([]string,error){
	rows,err := h.Db.Query("SELECT * FROM `containers` WHERE name LIKE "+"%"+term+"%"+" Limit ",limit)
	if err != nil{
		log.Println("image search db faile ",err.Error())
		return nil,err
	}
	images := []string{}
	defer rows.Close()
	for rows.Next(){
		var image string
		rows.Scan(&image)
		images = append(images,image)
	}
	return images,nil
}

func (h *Handler) experimentContentGetDB(experiment string) (string,error){
	var content string
	err := h.Db.QueryRow("SELECT content FROM `experiment` WHERE name = ",experiment).Scan(&content)
	if err != nil{
		log.Printf("fetch experiment %s content failed ",experiment,err.Error())
		return "",err
	}
	return content,nil
}