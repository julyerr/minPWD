package main

import "sync"

type User struct{
	UserName string `json:"user_name"`
	IsTeacher bool `json:"is_teacher"`
	ActiveSessions map[string]*Session `json:"-"`
	StoredSessions map[string]*Session `json:"-"`
	rw sync.Mutex
}

func (u *User) Lock(){
	u.rw.Lock()
}

func (u *User) Unlock(){
	u.rw.Unlock()
}

