package main

import (
	"context"
	"encoding/json"
	"flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron"
	"github.com/rs/cors"
	"github.com/spaceapi-community/go-spaceapi-validator-client"
	"goji.io"
	"goji.io/pat"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"time"
)

//go:generate go run scripts/generateOpenApi.go

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
		[]string{"route"},
	)
	spaceValidationGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "spaceapi_validation",
			Help: "Result of the validator",
		},
		[]string{"route", "attribute"},
	)
)

type entry struct {
	Url      string      `json:"url"`
	Valid    bool        `json:"valid"`
	LastSeen int64       `json:"lastSeen,omitempty"`
	ErrMsg   []string    `json:"errMsg,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
	ValidationResult	spaceapivalidatorclient.ValidateUrlV2Response	`json:"validationResult,omitempty"`
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
	prometheus.MustRegister(spaceValidationGauge)
	spaceApiDirectory = make(map[string]entry)

	directorySuccessfullyLoaded := loadPersistentDirectory()

	if rebuildDirectoryOnStart || !directorySuccessfullyLoaded {
		rebuildDirectory()
	}

	c := cron.New()
	err := c.AddFunc("@every 1m", func() {
		rebuildDirectory()
	})
	if err != nil {
		log.Printf("Can't start rebuilding directory cron %v", err)
	} else {
		c.Start()
	}

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
	log.Fatal(http.ListenAndServe(":8080", mux))
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
	_, err := writer.Write([]byte(openapi))
	if err != nil {
		writer.WriteHeader(500)
	}
}

func rebuildDirectory() {
	log.Println("rebuilding directory...")
	ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
	loadStaticFile()
	removeMissingStaticEntries()
	buildDirectory(ctx)
	persistDirectory()
	log.Println("rebuilding done.")
}

func loadStaticFile() {
	start := time.Now()
	defer staticFileScrapingTime.Set(time.Since(start).Seconds())

	resp, err := http.Get("https://raw.githubusercontent.com/spaceapi/directory/master/directory.json")
	if err != nil {
		log.Println(err)
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			panic(err)
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}

	var staticDirectory map[string]interface{}
	err = json.Unmarshal(body, &staticDirectory)

	var spaceUrls []string
	for _, value := range staticDirectory {
		spaceUrls = append(spaceUrls, value.(string))
	}

	spaceApiUrls = spaceUrls
	staticFileScrapCounter.Inc()
}

func removeMissingStaticEntries() {
	exists := false
	for directoryUrl := range spaceApiDirectory {
		exists = false
		for _, url := range spaceApiUrls {
			if directoryUrl == url {
				exists = true
			}
		}

		if !exists {
			delete(spaceApiDirectory, directoryUrl)
		}
	}
}

func persistDirectory() {
	log.Println("writing...")
	spaceApiDirectoryJson, err := json.Marshal(spaceApiDirectory)
	if err != nil {
		log.Println(err)
		panic("can't marshall api directory")
	}
	err = ioutil.WriteFile(spaceApiDirectoryFile, spaceApiDirectoryJson, 0644)
	if err != nil {
		log.Println(err)
		panic("can't write api directory to file")
	}
}

func loadPersistentDirectory() bool {
	log.Println("reading...")
	fileContent, err := ioutil.ReadFile(spaceApiDirectoryFile)
	if err != nil {
		log.Println(err)
		log.Println("can't read directory file, skipping...")
		return false
	}
	err = json.Unmarshal(fileContent, &spaceApiDirectory)
	if err != nil {
		log.Println(err)
		panic("can't unmarshal api directory")
	}

	return true
}

func buildDirectory(ctx context.Context) {
	c := make(chan entry, 32)
	for _, spaceApiUrl := range spaceApiUrls {
		go buildEntry(ctx, spaceApiUrl, c)
	}

	n := len(spaceApiUrls)
	for ; n > 0; n-- {
		v := <- c
		if v.LastSeen == 0 {
		    v.LastSeen = spaceApiDirectory[v.Url].LastSeen
		}

		spaceApiDirectory[v.Url] = v
	}

	var returnArray []map[string]interface{}
	for _, entry := range spaceApiDirectory {
		returnArray = append(returnArray, entry.Data)
	}

	generateFieldStatistic(returnArray)
	generateCountryStatistics(spaceApiDirectory)
}

func validateEntry(ctx context.Context, url string) (spaceapivalidatorclient.ValidateUrlV2Response, error) {
	apiClient := spaceapivalidatorclient.NewAPIClient(spaceapivalidatorclient.NewConfiguration())
	response, httpResp, err := apiClient.V2Api.V2ValidateURLPost(ctx, spaceapivalidatorclient.ValidateUrlV2{Url: url})
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 429 {
			log.Println("Too many requests, enhancing calm...")
			time.Sleep(time.Duration(rand.Intn(9) + 1) * time.Second)
			return validateEntry(ctx, url)
		}

		return spaceapivalidatorclient.ValidateUrlV2Response{}, err
	}

	var b2i = map[bool]float64{false: 0, true: 1}
	defer spaceValidationGauge.With(prometheus.Labels{"route": url, "attribute": "isHttps"}).Set(b2i[response.IsHttps])
	defer spaceValidationGauge.With(prometheus.Labels{"route": url, "attribute": "HttpsForward"}).Set(b2i[response.HttpsForward])
	defer spaceValidationGauge.With(prometheus.Labels{"route": url, "attribute": "Reachable"}).Set(b2i[response.Reachable])
	defer spaceValidationGauge.With(prometheus.Labels{"route": url, "attribute": "Cors"}).Set(b2i[response.Cors])
	defer spaceValidationGauge.With(prometheus.Labels{"route": url, "attribute": "ContentType"}).Set(b2i[response.ContentType])
	defer spaceValidationGauge.With(prometheus.Labels{"route": url, "attribute": "CertValid"}).Set(b2i[response.CertValid])
	defer spaceValidationGauge.With(prometheus.Labels{"route": url, "attribute": "Valid"}).Set(b2i[response.Valid])

	return response, nil
}

func buildEntry(ctx context.Context, url string, c chan entry) {
	ctx, _ = context.WithTimeout(ctx, 30 * time.Second)
	start := time.Now()

	entry := entry{
		Url: url,
	}

	response, err := validateEntry(ctx, url)
	defer spaceRequestSummary.With(prometheus.Labels{"route": url}).Observe(time.Since(start).Seconds())
	if err != nil {
		c <- entry
		return
	}

	entry.ValidationResult = response
	entry.Valid = response.Valid
	entry.LastSeen = time.Now().Unix()
	entry.Data = response.ValidatedJson

	c <- entry
	return
}
