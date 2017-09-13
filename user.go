package main


type User struct{
	Name string `json:"name"`
	IsTeacher bool `json:"is_teacher"`
	ImageName string `json:"image_name"`
	Experiment string `json:"experiment"`
	Sessions map[string][]string `json:"sessions"`
}

