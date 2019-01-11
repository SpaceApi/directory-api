package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron"
	"github.com/rs/cors"
	validator "github.com/spaceapi-community/go-spaceapi-validator"
	"goji.io"
	"goji.io/pat"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

var (
	staticFileScrapingTime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "spaceapi_scrape_static_file_time",
			Help: "Time used to load the static directory",
		})

	staticFileScrapCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "spaceapi_scrape_static_file_count",
			Help: "All the http requests!",
		})
	spaceRequestSummary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:   "spaceapi_response",
			Help:   "All the scraped spaces!",
			MaxAge: 4 * time.Hour,
		},
		[]string{"route", "error"},
	)
)

type entry struct {
	Url      string `json:"url"`
	Valid    bool   `json:"valid"`
	LastSeen int64  `json:"lastSeen,omitempty"`
	ErrMsg   string `json:"errMsg,omitempty"`
	Data	 interface{} `json:"data,omitempty"`
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
	prometheus.MustRegister(staticFileScrapingTime)
	prometheus.MustRegister(staticFileScrapCounter)
	prometheus.MustRegister(spaceRequestSummary)
	spaceApiDirectory = make(map[string]entry)

	loadPersistentDirectory()

	if rebuildDirectoryOnStart {
		rebuildDirectory()
	}

	c := cron.New()
	c.AddFunc("@hourly", func() {
		rebuildDirectory()
	})
	c.Start()

	co := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
	})

	mux := goji.NewMux()
	mux.Use(co.Handler)
	mux.Use(statisticMiddelware)

	mux.Handle(pat.Get("/metrics"), promhttp.Handler())
	mux.HandleFunc(pat.Get("/"), directory)
	mux.HandleFunc(pat.Get("/openapi.json"), openApi)

	log.Println("starting api...")
	log.Fatal(http.ListenAndServe(":8081", mux))
}

func directory(w http.ResponseWriter, _ *http.Request) {
	if err := json.NewEncoder(w).Encode(func() interface{} {
		var foo []entry
		for _, entry := range spaceApiDirectory {
			foo = append(foo, entry)
		}
		return foo
	}()); err != nil {
		panic(err)
	}
}

func openApi(writer http.ResponseWriter, _ *http.Request) {
	writer.Write([]byte(openapi))
}

func rebuildDirectory() {
	log.Println("rebuilding directory...")
	loadStaticFile()
	buildDirectory()
	persistDirectory()
	log.Println("rebuilding done.")
}

func loadStaticFile() {
	start := time.Now()
	defer staticFileScrapingTime.Set(time.Since(start).Seconds())

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
	staticFileScrapCounter.Inc()
}



func persistDirectory() {
	log.Println("writing...")
	spaceApiDirectoryJson, err := json.Marshal(spaceApiDirectory)
	if err != nil {
		log.Println(err)
		panic("can't marshall api directory")
	}
	err = ioutil.WriteFile(spaceApiDirectoryFile, []byte(spaceApiDirectoryJson), 0644)
	if err != nil {
		log.Println(err)
		panic("can't write api directory to file")
	}
}

func loadPersistentDirectory() {
	log.Println("reading...")
	fileContent, err := ioutil.ReadFile(spaceApiDirectoryFile)
	if err != nil {
		log.Println(err)
		log.Println("can't read directory file, skipping...")
		return
	}
	err = json.Unmarshal(fileContent, &spaceApiDirectory)
	if err != nil {
		log.Println(err)
		panic("can't unmarshal api directory")
	}
}

func buildDirectory() {
	var rawJsonArray [][]byte
	for _, spaceApiUrl := range spaceApiUrls {
		entry, rawJson := buildEntry(spaceApiUrl)

		if entry.Valid {
			rawJsonArray = append(rawJsonArray, rawJson)
		}

		if entry.LastSeen == 0 {
			entry.LastSeen = spaceApiDirectory[spaceApiUrl].LastSeen
		}

		spaceApiDirectory[spaceApiUrl] = entry
	}
	generateFieldStatistic(rawJsonArray)
}

func buildEntry(url string) (entry, []byte) {
	start := time.Now()
	var spaceError = ""

	entry := entry{
		Url: url,
	}

	resp, err := http.Get(url)
	if err != nil {
		entry.ErrMsg = err.Error()
		spaceError = "http"
		defer spaceRequestSummary.With(prometheus.Labels{"route": url, "error": spaceError}).Observe(time.Since(start).Seconds())
		return entry, nil
	} else if resp.StatusCode != 200 {
		defer spaceRequestSummary.With(prometheus.Labels{"route": url, "error": spaceError + resp.Status}).Observe(time.Since(start).Seconds())
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		entry.ErrMsg = err.Error()
		spaceError = "body"
		defer spaceRequestSummary.With(prometheus.Labels{"route": url, "error": spaceError}).Observe(time.Since(start).Seconds())
		return entry, nil
	}

	validJson := json.Valid(body)
	if validJson == false {
		entry.ErrMsg = "Server doesn't provide valid json"
		spaceError = "json"
		defer spaceRequestSummary.With(prometheus.Labels{"route": url, "error": spaceError}).Observe(time.Since(start).Seconds())
		return entry, nil
	}

	result, err := validator.Validate(string(body[:]))
	if err != nil {
		entry.ErrMsg = err.Error()
		spaceError = "validation"
		defer spaceRequestSummary.With(prometheus.Labels{"route": url, "error": spaceError}).Observe(time.Since(start).Seconds())
		return entry, nil
	}

	entry.Valid = result.Valid
	if result.Valid == false {
		spaceError = "invalid"
		entry.ErrMsg = func() string {
			var errorMsgs []string
			for _, err := range result.Errors {
				errorMsgs = append(errorMsgs, fmt.Sprintf("%s %s %s", err.Context, err.Field, err.Description))
			}

			return strings.Join(errorMsgs, ", ")
		}()
	}

	var respJson map[string]interface{}
	json.Unmarshal(body, &respJson)

	entry.LastSeen = time.Now().Unix()
	entry.Data = respJson

	defer spaceRequestSummary.With(prometheus.Labels{"route": url, "error": spaceError}).Observe(time.Since(start).Seconds())
	return entry, body
}