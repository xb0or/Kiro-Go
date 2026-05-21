FROM golang:1.23-bookworm AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux GOFLAGS=-trimpath
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /kiro-go .

FROM debian:bookworm-slim
LABEL "language"="go"
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /kiro-go .
COPY --from=build /src/web ./web
RUN mkdir -p /app/data
# Zeabur / generic container platforms inject $PORT; main.go reads it and falls back to 8080.
ENV PORT=8080
ENV CONFIG_PATH=/app/data/config.json
EXPOSE 8080
CMD ["./kiro-go"]
