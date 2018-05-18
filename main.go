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

	"github.com/rs/cors"

	"goji.io"
	"goji.io/pat"
	"strconv"
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

	fmt.Println(len(spaceApiDirectory))

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

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
	})

	mux := goji.NewMux()
	mux.Use(c.Handler)
	mux.HandleFunc(pat.Get("/"), serveV1)
	mux.HandleFunc(pat.Get("/v1"), serveV1)
	mux.HandleFunc(pat.Get("/v2"), serveV2)

	log.Fatal(http.ListenAndServe(":8080", mux))
}

func getFilter(r *http.Request) (bool, bool) {
	validFilterQuery := r.URL.Query().Get("valid")

	if validFilterQuery == "all" {
		return false, true
	} else if validFilterQuery != "" {
		validFilter, err := strconv.ParseBool(validFilterQuery)
		if err != nil {
			return true, false
		}

		return validFilter, false
	}

	return true, false
}

func serveV1 (w http.ResponseWriter, r *http.Request) {
	validFilter, noFilter := getFilter(r)
	if err := json.NewEncoder(w).Encode(func() interface{} {
		foo := make(map[string]string)
		for _, entry := range spaceApiDirectory {
			if entry.Valid == validFilter || noFilter == true {
				foo[entry.Space] = entry.Url
			}
		}
		return foo
	}()); err != nil {
		panic(err)
	}
}

func serveV2 (w http.ResponseWriter, r *http.Request) {
	validFilter, noFilter := getFilter(r)
	if err := json.NewEncoder(w).Encode(func() []entry {
		var foo []entry
		for _, entry := range spaceApiDirectory {
			if entry.Valid == validFilter || noFilter == true {
				foo = append(foo, entry)
			}
		}
		return foo
	}()); err != nil {
		panic(err)
	}
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
	if validJson == false {
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
	} else {
		entry.Space = url
	}
	entry.LastSeen = time.Now().Unix()

	return entry
}