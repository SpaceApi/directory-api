package main

import (
	"errors"
	"github.com/felixge/httpsnoop"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"sort"
	"strconv"
)

var (
	spaceFieldGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "spaceapi_field",
			Help: "Fields used from the spec",
		},
		[]string{"version", "space", "field"},
	)
	httpRequestSummary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "spaceapi_http_requests",
			Help: "All the http requests!",
		},
		[]string{"method", "route", "code"},
	)
	statistics map[string]map[string][]string
)

func init() {
	prometheus.MustRegister(spaceFieldGauge)
	prometheus.MustRegister(httpRequestSummary)
}

func generateFieldStatistic(jsonArray []map[string]interface{}) {
	newStats :=  make(map[string]map[string][]string)
	for _, value := range jsonArray {
		apiVersion, space, fields, err := getNewStats(value)
		if err == nil {
			if _, ok := newStats[space]; !ok {
				newStats[space] = make(map[string][]string)
			}

			newStats[space][apiVersion] = fields
		}
	}

	for spaceName, value := range newStats {
		for version, fields := range value {
			for _, field := range fields {
				spaceFieldGauge.With(prometheus.Labels{"version": version, "space": spaceName, "field": field}).Set(1)
			}
		}
	}

	for spaceName, value := range statistics {
		if _, ok := newStats[spaceName]; !ok {
			for version, fields := range value {
				for _, field := range fields {
					deleteGauge(version, spaceName, field)
				}
			}
		} else {
			for version, fields := range value {
				if _, ok := newStats[spaceName][version]; !ok {
					for _, field := range fields {
						deleteGauge(version, spaceName, field)
					}
				} else {
					for _, field := range fields {
						if sort.SearchStrings(newStats[spaceName][version], field) == 0 {
							deleteGauge(version, spaceName, field)
						}
					}
				}
			}
		}
	}

	statistics = newStats
}

func deleteGauge(version string, spaceName string, field string) {
	spaceFieldGauge.With(prometheus.Labels{"version": version, "space": spaceName, "field": field}).Set(0)
}

func statisticMiddelware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		m := httpsnoop.CaptureMetrics(inner, w, r)
		httpRequestSummary.With(prometheus.Labels{"method": r.Method, "route": r.URL.Path, "code": strconv.Itoa(m.Code)}).Observe(m.Duration.Seconds())
	}
	return http.HandlerFunc(mw)
}

func getNewStats(value interface{}) (string, string, []string, error) {
	castedValue := value.(map[string]interface{})
	apiVersion, ok := castedValue["api"].(string)
	space, ok2 := castedValue["space"].(string)

	if ok && ok2 {
		return apiVersion, space, flatten(castedValue, ""), nil
	} else {
		return "", "", nil, errors.New("api or space doesn't exist")
	}
}

func flatten(from map[string]interface{}, prepend string) []string {
	var to []string
	for key, value := range from {
		obj, isObject := value.(map[string]interface{})

		if isObject {
			to = append(to, flatten(obj, prepend+"/"+key)...)
		} else {
			to = append(to, prepend+"/"+key)
		}
	}

	return to
}
