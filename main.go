package main

import (
	"net/http"
	"log"
	"io/ioutil"
	"strings"
	"fmt"
	"time"
	"encoding/json"
	validator "github.com/spaceapi-community/go-spaceapi-validator"
	"github.com/robfig/cron"

	"goji.io"
	"goji.io/pat"
)

type entry struct {
	Url string `json:"url"`
	Valid bool `json:"valid"`
	Space string `json:"space,omitempty"`
	LastSeen int64 `json:"lastSeen,omitempty"`
	ErrMsg string `json:"errMsg,omitempty"`
}

var spaceApiUrls []string
var spaceApiDirectory map[string]entry

func init() {
	spaceApiDirectory = make(map[string]entry)

	loadStaticFile()
	buildDirectory()

	c := cron.New()
	c.AddFunc("@hourly", func() {
		log.Println("update directory")
		loadStaticFile()
		buildDirectory()
		log.Println("directory updated")
	})
	c.Start()
}

func main() {
	log.Println("started directory...")

	mux := goji.NewMux()
	mux.HandleFunc(pat.Get("/v0"), func (w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(func() []entry {
			var foo []entry
			for _, entry := range spaceApiDirectory {
				foo = append(foo, entry)
			}
			return foo
		}()); err != nil {
			panic(err)
		}
	})

	log.Fatal(http.ListenAndServe(":8080", mux))
}

func loadStaticFile() {
	resp, err := http.Get("https://raw.githubusercontent.com/fixme-lausanne/OpenSpaceDirectory/master/directory.json")
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}

	var staticDirectory map[string]interface{}
	json.Unmarshal(body, &staticDirectory)

	var spaceUrls []string
	for _, value := range staticDirectory {
		spaceUrls = append(spaceUrls, value.(string))
	}

	spaceApiUrls = spaceUrls
}

func buildDirectory() {
	for _, spaceApiUrl := range spaceApiUrls {
		entry := buildEntry(spaceApiUrl)

		if entry.LastSeen == 0 {
			entry.LastSeen = spaceApiDirectory[spaceApiUrl].LastSeen
		}

		spaceApiDirectory[spaceApiUrl] = entry
	}
}

func buildEntry(url string) entry {
	entry := entry{
		Url: url,
	}

	resp, err := http.Get(url)
	if err != nil {
		entry.ErrMsg = err.Error()
		return entry
	}
	defer resp.Body.Close()


	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		entry.ErrMsg = err.Error()
		return entry
	}

	validJson := json.Valid(body)
	if !validJson {
		entry.ErrMsg = "Server doesn't provid valid json"
		return entry
	}

	result, err := validator.Validate(string(body[:]))
	if err != nil {
		entry.ErrMsg = err.Error()
		return entry
	}

	entry.Valid = result.Valid
	if result.Valid == false {
		entry.ErrMsg = func() string {
			var errorMsgs []string
			for _, error := range result.Errors {
				errorMsgs = append(errorMsgs, fmt.Sprintf("%s %s %s", error.Context, error.Field, error.Description))
			}

			return strings.Join(errorMsgs, ", ")
		}()
	}

	var respJson map[string]interface{}
	json.Unmarshal(body, &respJson)

	if respJson["space"] != nil {
		entry.Space = respJson["space"].(string)
	}
	entry.LastSeen = time.Now().Unix()

	return entry
}