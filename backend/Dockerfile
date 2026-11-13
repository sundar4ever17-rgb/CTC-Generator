FROM golang:1.27.4-bookworm AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /ctc-api ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends chromium ca-certificates fonts-liberation fonts-noto-core && rm -rf /var/lib/apt/lists/*
RUN useradd -m -u 10001 ctc
WORKDIR /app
COPY --from=build /ctc-api /app/ctc-api
ENV PORT=8080 CHROME_PATH=/usr/bin/chromium
USER ctc
EXPOSE 8080
CMD ["/app/ctc-api"]
