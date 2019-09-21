FROM golang:1.12-alpine as builder
RUN apk --no-cache add git
WORKDIR /app
COPY . .
RUN go get -d  ./...
RUN go generate
RUN go install  ./...

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /go/bin/collector /usr/local/bin/collector
EXPOSE 8080
VOLUME /srv/spaceapi

RUN adduser app -S -u 142
USER app

CMD ["collector", "-storage", "/srv/spaceapi/spaceapiDirectory.json"]
