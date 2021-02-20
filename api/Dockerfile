FROM golang:1.16-alpine as builder
RUN apk --no-cache add git
WORKDIR /app
COPY . .
RUN go get -d  ./...
RUN go generate
RUN go install  ./...

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /go/bin/api /usr/local/bin/api
EXPOSE 8080

RUN adduser app -S -u 142
USER app

CMD ["api"]
