package main

var openapi = `{
  "openapi": "3.0.0",
  "info": {
    "description": "This is the spaceApi directory",
    "version": "0.0.1",
    "title": "SpaceApi Directory",
    "termsOfService": "https://example.com/daemon/terms",
    "contact": {
      "email": "spaceapi-team@chaospott.de"
    },
    "license": {
      "name": "Apache 2.0",
      "url": "http://www.apache.org/licenses/LICENSE-2.0.html"
    }
  },
  "paths": {
    "/v1": {
      "get": {
        "summary": "",
        "parameters": [
          {
            "in": "query",
            "name": "valid",
            "schema": {
              "type": "string",
              "enum": [
                "all",
                "true",
                "false"
              ],
              "default": "true"
            },
            "description": "Filter for valid endpoints"
          }
        ],
        "responses": {
          "200": {
            "description": "successful operation",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/DirectoryV1"
                }
              }
            }
          },
          "500": {
            "description": "something went wrong"
          }
        }
      }
    },
    "/v2": {
      "get": {
        "summary": "",
        "parameters": [
          {
            "in": "query",
            "name": "valid",
            "schema": {
              "type": "string",
              "enum": [
                "all",
                "true",
                "false"
              ],
              "default": "true"
            },
            "description": "Filter for valid endpoints"
          }
        ],
        "responses": {
          "200": {
            "description": "successful operation",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/DirectoryV2"
                }
              }
            }
          },
          "500": {
            "description": "something went wrong"
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "DirectoryV1": {
        "type": "object",
        "properties": {}
      },
      "DirectoryV2": {
        "description": "SpaceAPI Directory 0.2",
        "type": "object",
        "properties": {
          "items": {
            "description": "List of directory entries",
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "url": {
                  "description": "url to the spaceapi file",
                  "type": "string"
                },
                "valid": {
                  "description": "indicates if the provided file is valid",
                  "type": "boolean"
                },
                "space": {
                  "description": "The name of the space",
                  "type": "string"
                },
                "lastSeen": {
                  "description": "when we've seen the endpoint the last time (doesn't have to be valid, but the url was reachable and provided valid json)",
                  "type": "number"
                },
                "errMsg": {
                  "description": "provided if we found an error with that specific endpoint",
                  "type": "string"
                }
              },
              "required": [
                "url",
                "valid"
              ]
            }
          }
        }
      }
    }
  },
  "externalDocs": {
    "description": "Find out more about spaceApi directory",
    "url": "https://spaceapi.net"
  }
}`