FROM golang:1.27 AS build-stage

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN GOOS=linux go build -o /docker-server cmd/userserver/main.go

FROM build-stage AS run-test-stage
RUN go test -v ./...

FROM alpine:3.24.3 AS build-release-stage

WORKDIR /

COPY --from=build-stage /docker-server /docker-server

EXPOSE 8080

CMD ["/docker-server"]
