FROM golang:1.22-alpine3.19 AS builder

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /build

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY main.go .

RUN go build -ldflags="-s -w -extldflags '-static'" -o annie-api main.go

FROM python:3.12-alpine3.19

RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    tzdata \
    su-exec

RUN addgroup -S anniegroup && adduser -S annieuser -G anniegroup
WORKDIR /app

ENV VIRTUAL_ENV=/opt/venv
RUN python3 -m venv $VIRTUAL_ENV
ENV PATH="$VIRTUAL_ENV/bin:$PATH"

RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir "yt-dlp>=2024.03.10" && \
    pip install --no-cache-dir https://github.com/coletdjnz/yt-dlp-youtube-oauth2/archive/refs/heads/master.zip

RUN mkdir -p /app/config && \
    chown -R annieuser:anniegroup /app /opt/venv

COPY --from=builder --chown=annieuser:anniegroup /build/annie-api /app/annie-api

RUN chmod 500 /app/annie-api

ENV PORT=7860 \
    GIN_MODE=release

USER annieuser

EXPOSE 7860

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:7860/health || exit 1

ENTRYPOINT ["/app/annie-api"]
