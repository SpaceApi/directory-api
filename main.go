package main

import (
	"log"
)

type entry struct {
	Url      string `json:"url"`
	Valid    bool   `json:"valid"`
	Space    string `json:"space,omitempty"`
	LastSeen int64  `json:"lastSeen,omitempty"`
	ErrMsg   string `json:"errMsg,omitempty"`
}

var spaceApiDirectory map[string]entry
var spaceApiUrls []string

func main() {
	log.Println("started directory daemon...")
	initApi()
}
