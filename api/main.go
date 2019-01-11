package main

import (
	"encoding/json"
	"github.com/felixge/httpsnoop"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"goji.io"
	"goji.io/pat"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
)

//go:generate go run scripts/generate.go

type entry struct {
	Url      string `json:"url"`
	Valid    bool   `json:"valid"`
	Space	string	`json:"space,omitempty"`
	LastSeen int64  `json:"lastSeen,omitempty"`
	ErrMsg   string `json:"errMsg,omitempty"`
}

type collectorEntry struct {
	Url      string `json:"url"`
	Valid    bool   `json:"valid"`
	LastSeen int64  `json:"lastSeen,omitempty"`
	ErrMsg   string `json:"errMsg,omitempty"`
	Data	 interface{} `json:"data,omitempty"`
}

var (
	httpRequestSummary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "spaceapi_http_requests",
			Help: "All the http requests!",
		},
		[]string{"method", "route", "code"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestSummary)
}

func main() {
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
	})

	mux := goji.NewMux()
	mux.Use(c.Handler)
	mux.Use(statisticMiddelware)

	mux.Handle(pat.Get("/metrics"), promhttp.Handler())

	mux.HandleFunc(pat.Get("/"), serveV1)
	mux.HandleFunc(pat.Get("/v1"), serveV1)
	mux.HandleFunc(pat.Get("/v2"), serveV2)
	mux.HandleFunc(pat.Get("/openapi.json"), openApi)

	log.Println("starting api...")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func openApi(writer http.ResponseWriter, request *http.Request) {
	writer.Write([]byte(openapi))
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

func serveV1(w http.ResponseWriter, r *http.Request) {
	validFilter, noFilter := getFilter(r)
	if err := json.NewEncoder(w).Encode(func() interface{} {
		var foo []string
		for _, entry := range getDirectory() {
			if entry.Valid == validFilter || noFilter == true {
				foo = append(foo, entry.Url)
			}
		}
		return foo
	}()); err != nil {
		panic(err)
	}
}

func serveV2(w http.ResponseWriter, r *http.Request) {
	validFilter, noFilter := getFilter(r)
	if err := json.NewEncoder(w).Encode(func() []entry {
		var foo []entry
		for _, collectorEntry := range getDirectory() {
			if collectorEntry.Valid == validFilter || noFilter == true {
				foo = append(foo, entry{
					collectorEntry.Url,
					collectorEntry.Valid,
					"",
					collectorEntry.LastSeen,
					collectorEntry.ErrMsg,
				})
			}
		}
		return foo
	}()); err != nil {
		panic(err)
	}
}

func statisticMiddelware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		m := httpsnoop.CaptureMetrics(inner, w, r)
		httpRequestSummary.With(prometheus.Labels{"method": r.Method, "route": r.URL.Path, "code": strconv.Itoa(m.Code)}).Observe(m.Duration.Seconds())
	}
	return http.HandlerFunc(mw)
}

func getDirectory() []collectorEntry {
	resp, err := http.Get("http://localhost:8081/")
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}

	var staticDirectory []collectorEntry
	err = json.Unmarshal(body, &staticDirectory)

	return staticDirectory

}