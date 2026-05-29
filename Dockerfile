# builder 阶段始终运行在构建机原生平台（amd64），用 Go 交叉编译目标平台二进制
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /kiro-go .

FROM debian:bookworm-slim
LABEL "language"="go"
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /kiro-go .
COPY --from=builder /app/web ./web
RUN mkdir -p /app/data
# Zeabur / generic container platforms inject $PORT; main.go reads it and falls back to 8080.
ENV PORT=8080
ENV CONFIG_PATH=/app/data/config.json
EXPOSE 8080
CMD ["./kiro-go"]
