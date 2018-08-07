package main

import (
					"os"
		"io/ioutil"
)

func main() {
  out, _ := os.Create("openapi.go")

  openApiContent, _ := ioutil.ReadFile("openapi.json")

  out.Write([]byte("package main\n\nvar openapi = `"))
  out.Write(openApiContent)
  out.Write([]byte("`"))
}