FROM golang:1.23 AS build
WORKDIR /src
COPY . .
RUN go build -o /kiro-go .

FROM debian:bookworm-slim
LABEL "language"="go"
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /kiro-go .
COPY --from=build /src/web ./web
RUN mkdir -p /app/data
EXPOSE 8080
CMD ["./kiro-go"]
