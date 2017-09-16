package main


type User struct{
	Name string `json:"name"`
	IsTeacher bool `json:"is_teacher"`
	Sessions map[string]*EachSession `json:"sessions"`
}

type EachSession struct{
	ImageName string `json:"image_name"`
	Experiment string `json:"experiment"`
	Resumed bool `json:"resumed"`
	Instances map[string]string `json:"instances"`
}
