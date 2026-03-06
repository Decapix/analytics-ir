FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/event-collector ./cmd/event-collector

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/event-collector /event-collector
EXPOSE 8080
ENTRYPOINT ["/event-collector"]
