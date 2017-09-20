package main

import (
	"database/sql"
	"log"
	_ "github.com/go-sql-driver/mysql"
)

func DBInit() (db *sql.DB,err error){
	db,err = sql.Open("mysql",DBSchm)
	return
}

func StoreSessionDB(db *sql.DB,s *Session ,content string) error{
	stmt,err := db.Prepare("INSERT INTO `sessions`(`sessionId`,`sessionComment`,`name`,`isTeacher`,`experiment`,`image`) VALUES(?,?,?,?,?,?)")
	if CheckError(err) {
		return err
	}
	_, err = stmt.Exec(s.SessionId,content,s.User.UserName,s.User.IsTeacher,s.ExperimentName,s.ImageName)
	if CheckError(err) {
		return err
	}
	for _,v := range s.Instances{
		stmt , err := db.Prepare("INSERT INTO `images` (`imageId`,`sessionId`,`hostname`,`isMount`,`mountSize`,`memSize`) VALUES(?,?,?,?,?,?)")
		_,err =stmt.Exec(v.Config.ImageName,s.SessionId,v.Config.Hostname,v.Config.IsMount,v.Config.MountSize,v.Config.MemSize)
		if CheckError(err) {
			return err
		}
	}
	return nil
}

func DeleteSessionDB(db *sql.DB,s *Session) error{
	_,err := db.Exec("DELETE FROM `images` WHERE `sessionId`=?",s.SessionId)
	if CheckError(err){
		return err
	}
	_,err = db.Exec("DELETE FROM `sessions` WHERE `sessionId`=?",s.SessionId)
	if CheckError(err){
		return err
	}
	return nil
}

func ImageSearchDB(db *sql.DB,term string,limit int) ([]string,error){
	log.Println("db searching,term,limit Num",term,limit)
	if limit == 0{
		limit = 5
	}
	rows,err := db.Query("SELECT * FROM `containers` WHERE name LIKE \"%"+term+"%\" Limit ? ",limit)
	if CheckError(err){
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

func ContentGetDB(db *sql.DB,experimentName string) (string,error){
	var content string
	err := db.QueryRow("SELECT content FROM `experiments` WHERE name = ?",experimentName).Scan(&content)
	if CheckError(err){
		return "",err
	}
	return content,nil
}

func UserFromDB(h *Handler) error{
	rows,err := h.DB.Query("SELECT sessionId,sessionComment,name,isTeacher,experiment,image FROM `sessions`")
	if CheckError(err){
		return err
	}
	defer rows.Close()
	for rows.Next(){
		var sessionId,sessionComment,name,image,experiment string
		var isTeacher bool
		rows.Scan(&sessionId,&sessionComment,&name,&isTeacher,&experiment,&image)
		log.Println("fetch user info :",name)
		u := h.Users[name]
		if u== nil{
			u = &User{UserName:name,IsTeacher:isTeacher}
			u.StoredSessions=make(map[string]*Session)
			h.Users[name]=u
		}
		instances := make(map[string]*Instance)

		rows1,err := h.DB.Query("SELECT imageId,sessionId,hostname,isMount,mountSize,memSize from `images` WHERE sessionId = ?",sessionId)
		if CheckError(err) {
			return err
		}
		defer rows1.Close()

		for rows1.Next(){
			var imageId,sessionId,hostname string
			var isMount bool
			var mountSize,memSize int64
			rows1.Scan(&imageId,&sessionId,&hostname,&isMount,&mountSize,&memSize)
			log.Printf("load image %s from session %s ",imageId,sessionId)
			instance := &Instance{
				Config:&InstanceConfig{
					Hostname:hostname,
					ImageName:imageId,
					IsMount:isMount,
					MountSize:mountSize,
					MemSize:memSize,
				},
			}
			instances[hostname]=instance
		}

		s:= &Session{
			SessionId:sessionId,
			ExperimentName:experiment,
			ImageName:image,
			User:h.Users[name],
			Instances:instances,
		}
		if h.Users[name].StoredSessions == nil{
			h.Users[name].StoredSessions = make(map[string]*Session)
		}
		h.Users[name].StoredSessions[sessionId]=s
	}
	return nil
}
