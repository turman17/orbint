FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /ingest ./cmd/ingest

FROM alpine:3.21
COPY --from=build /ingest /ingest
ENTRYPOINT ["/ingest"]
