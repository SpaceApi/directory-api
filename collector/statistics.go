package main

import (
	"errors"
	"github.com/codingsince1985/geo-golang/openstreetmap"
	"github.com/felixge/httpsnoop"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"net/http"
	"reflect"
	"strconv"
)

var (
	spaceFieldGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "spaceapi_field",
			Help: "Fields used from the spec",
		},
		[]string{"version", "route", "field"},
	)
	spaceCountryGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "spaceapi_country",
			Help: "Countries spaces are from",
		},
		[]string{"country", "route"},
	)
	httpRequestSummary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "spaceapi_http_requests",
			Help: "All the http requests!",
		},
		[]string{"method", "route", "code"},
	)
	latLonCountry map[float64]map[float64]string
)

func init() {
	latLonCountry = make(map[float64]map[float64]string)
	prometheus.MustRegister(spaceFieldGauge)
	prometheus.MustRegister(spaceCountryGauge)
	prometheus.MustRegister(httpRequestSummary)
}

func generateCountryStatistics(entries map[string]entry) {
	spaceCountryGauge.Reset()
	for _, value := range entries {
		if value.Data["location"] != nil && value.Data["url"] != nil && value.Valid {
			val := reflect.ValueOf(value.Data["location"])

			var latVal, lonVal reflect.Value

			for _, foo := range val.MapKeys() {
				if foo.String() == "lat" {
					latVal = val.MapIndex(foo)
				}

				if foo.String() == "lon" {
					lonVal = val.MapIndex(foo)
				}
			}

			countryCode, err := getCountryCodeForLatLong(latVal.Interface().(float64), lonVal.Interface().(float64))
			if err == nil {
				spaceCountryGauge.With(prometheus.Labels{"country": countryCode, "route": value.Data["url"].(string)}).Inc()
			} else {
				log.Printf("%v\n", err)
			}
		}
	}
}

func getCountryCodeForLatLong(lat, long float64) (string, error) {
	if _, ok := latLonCountry[lat]; ok {
		if _, ok := latLonCountry[lat][long]; ok {
			return latLonCountry[lat][long], nil
		}
	} else {
		latLonCountry[lat] = make(map[float64]string)
	}

	geocoder := openstreetmap.Geocoder()
	address, err := geocoder.ReverseGeocode(lat, long)
	if err != nil {
		log.Printf("Unable to geocode lat: %v, long: %v", lat, long)
		return "", err
	}

	latLonCountry[lat][long] = address.CountryCode

	return address.CountryCode, nil
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

	spaceFieldGauge.Reset()
	for spaceName, value := range newStats {
		for version, fields := range value {
			for _, field := range fields {
				spaceFieldGauge.With(prometheus.Labels{"version": version, "route": spaceName, "field": field}).Set(1)
			}
		}
	}
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
	space, ok2 := castedValue["url"].(string)

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
