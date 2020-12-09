package main

import (
	"errors"
	"fmt"
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
		[]string{"field"},
	)
	spaceVersionGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "spaceapi_version",
			Help: "Versions used in the directory",
		},
		[]string{"version"},
	)
	spaceCountryGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "spaceapi_country",
			Help: "Countries spaces are from",
		},
		[]string{"country"},
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
	prometheus.MustRegister(spaceVersionGauge)
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
				spaceCountryGauge.With(prometheus.Labels{"country": countryCode}).Inc()
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
		return "", fmt.Errorf("unable to geocode lat: %v, long: %v, error was: %v", lat, long, err)
	}

	latLonCountry[lat][long] = address.CountryCode

	return address.CountryCode, nil
}

func generateFieldStatistic(jsonArray map[string]entry) {
	newStats :=  make(map[string][]string)

	spaceVersionGauge.Reset()
	for _, value := range jsonArray {
		apiVersions, fields, err := getNewStats(value.Data)
		if err == nil {
			newStats[value.Url] = fields

			for _, version := range apiVersions {
				spaceVersionGauge.With(prometheus.Labels{"version": version}).Inc()
			}
		}
	}

	spaceFieldGauge.Reset()
	for _, fields := range newStats {
		for _, field := range fields {
			spaceFieldGauge.With(prometheus.Labels{"field": field}).Inc()
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

func getNewStats(value interface{}) ([]string, []string, error) {
	castedValue := value.(map[string]interface{})

	var versions []string
	apiVersion, ok := castedValue["api"].(string)
	if ok {
		versions = append(versions, apiVersion)
	}

	apiCompatibility, ok := castedValue["api_compatibility"].([]interface{})
	if ok {
		for _, version := range apiCompatibility {
			versions = append(versions, version.(string))
		}
	}

	if len(versions) > 0 {
		return versions, flatten(castedValue, ""), nil
	} else {
		return []string{}, nil, errors.New("api or space doesn't exist")
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
