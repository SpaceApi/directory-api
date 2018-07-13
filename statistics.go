package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"encoding/json"
	"net/http"
	"github.com/felixge/httpsnoop"
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
)

func init() {
	prometheus.MustRegister(spaceFieldGauge)
	prometheus.MustRegister(httpRequestSummary)
}

func GenerateFieldStatistic(jsonArray [][]byte) {
	for _, value := range jsonArray {
		bar(value)
	}
}

func statisticMiddelware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		m := httpsnoop.CaptureMetrics(inner, w, r)
		httpRequestSummary.With(prometheus.Labels{"method": r.Method, "route": r.URL.Path, "code": strconv.Itoa(m.Code)}).Observe(m.Duration.Seconds())
	}
	return http.HandlerFunc(mw)
}

func bar(content []byte) {
	var value interface{}
	err := json.Unmarshal(content, &value)

	if err == nil {
		castedValue := value.(map[string]interface{})
		apiVersion, ok := castedValue["api"].(string)
		space, ok2 := castedValue["space"].(string)

		if ok && ok2 {
			for _, field := range flatten(castedValue, "") {
				spaceFieldGauge.With(prometheus.Labels{"version": apiVersion, "space": space, "field": field}).Set(1)
			}
		}
	}
}

func flatten(from map[string]interface{}, prepend string) []string {
	var to []string
	for key, value := range from {
		obj, isObject := value.(map[string]interface{})

		if isObject {
			to = append(to, flatten(obj, prepend + "/" + key)...)
		} else {
			to = append(to, prepend + "/" + key)
		}
	}

	return to
}