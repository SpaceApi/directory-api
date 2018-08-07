package main

import (
	"net/http"
	"strconv"
	"encoding/json"
	"github.com/rs/cors"
	"goji.io"
	"goji.io/pat"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
)

func initApi() {
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