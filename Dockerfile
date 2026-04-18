# ==========================================
# STAGE 1: Go Builder (Go 1.26 + Alpine 3.23)
# ==========================================
FROM golang:1.26-alpine3.23 AS builder

# إعداد بيئة البناء الحديثة
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /build

RUN apk add --no-cache git ca-certificates

# نسخ كل الملفات وتوليد go.sum داخلياً لتفادي أي أخطاء
COPY . .
RUN go mod tidy

# بناء الملف التنفيذي بمعايير 2026 (حجم أصغر، أمان أعلى، بدون مسارات وهمية)
RUN go build -trimpath -ldflags="-s -w -extldflags '-static'" -o annie-api main.go

# ==========================================
# STAGE 2: Runtime (Python 3.14 + Alpine 3.23)
# ==========================================
FROM python:3.14-alpine3.23

# تسطيب أحدث FFmpeg والاعتماديات الأساسية
RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    tzdata

# إنشاء مستخدم للحماية
RUN addgroup -S anniegroup && adduser -S annieuser -G anniegroup
WORKDIR /app

# إعداد البيئة الافتراضية
ENV VIRTUAL_ENV=/opt/venv
RUN python3 -m venv $VIRTUAL_ENV
ENV PATH="$VIRTUAL_ENV/bin:$PATH"

# تسطيب أحدث إصدارات yt-dlp وبلجن التخطي
RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir "yt-dlp>=2026.3.17" && \
    pip install --no-cache-dir https://github.com/coletdjnz/yt-dlp-youtube-oauth2/archive/refs/heads/master.zip

RUN mkdir -p /app/config && \
    chown -R annieuser:anniegroup /app /opt/venv

# نقل السيرفر الجاهز من مرحلة البناء
COPY --from=builder --chown=annieuser:anniegroup /build/annie-api /app/annie-api

RUN chmod 500 /app/annie-api

ENV PORT=7860 \
    GIN_MODE=release

USER annieuser

EXPOSE 7860

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:7860/health || exit 1

ENTRYPOINT ["/app/annie-api"]
