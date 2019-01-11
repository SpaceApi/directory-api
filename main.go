package main

import (
	"log"
	"flag"
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
var spaceApiDirectoryFile string
var rebuildDirectoryOnStart bool

func init() {
	flag.StringVar(
		&spaceApiDirectoryFile,
		"storage",
		"spaceApiDirectory.json",
		"Path to the file for persistent storage",
	)

	flag.BoolVar(
		&rebuildDirectoryOnStart,
		"rebuildDirectory",
		false,
		"Rebuild directory on startup",
	)
	flag.Parse()
}

func main() {
	log.Println("starting directory daemon...")
	initBuildDirectory()
	initApi()
}
