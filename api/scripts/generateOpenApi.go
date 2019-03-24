package main

import (
	"io/ioutil"
	"os"
)

func main() {
	out, _ := os.Create("openapi.go")

	openApiContent, _ := ioutil.ReadFile("openapi.json")

	_, err := out.Write(
		append(
			append(
				[]byte("package main\n\nvar openapi = `"),
				openApiContent...
			),
			[]byte("`")...
		),
	)
	if err != nil {
		panic(err)
	}
}
