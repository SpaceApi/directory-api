package main

import (
	"time"
	"net/http"
	"github.com/prometheus/client_golang/prometheus"
	"io/ioutil"
	"encoding/json"
	validator "github.com/spaceapi-community/go-spaceapi-validator"
	"fmt"
	"strings"
	"log"
	"github.com/robfig/cron"
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
			Name: "spaceapi_response",
			Help: "All the scraped spaces!",
		},
		[]string{"route", "error"},
	)
)

//go:generate go run scripts/generate.go

func init() {
	prometheus.MustRegister(staticFileScrapingTime)
	prometheus.MustRegister(staticFileScrapCounter)
	prometheus.MustRegister(spaceRequestSummary)
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

	GenerateFieldStatistic(rawJsonArray)
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
		entry.ErrMsg = "Server doesn't provid valid json"
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

	defer spaceRequestSummary.With(prometheus.Labels{"route": url, "error": spaceError}).Observe(time.Since(start).Seconds())
	return entry, body
}