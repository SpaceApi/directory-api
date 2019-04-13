FROM golang:1.11-alpine as builder
RUN apk --no-cache add git
WORKDIR /go/src/app
COPY . .
RUN go get -d  ./...
RUN go generate
RUN go install  ./...

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /go/bin/app .
EXPOSE 8080
VOLUME /srv/spaceapi

RUN adduser app -S -u 142
USER app

CMD ["./app", "-storage", "/srv/spaceapi/spaceapiDirectory.json"]