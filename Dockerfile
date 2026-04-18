# STAGE 1: بناء ملف Go التنفيذي بأحدث إصدار 1.26
FROM golang:1.26-alpine3.23 AS builder
WORKDIR /build
RUN apk add --no-cache git
COPY . .
RUN go mod tidy
RUN go build -trimpath -ldflags="-s -w -extldflags '-static'" -o annie-api main.go

# STAGE 2: بيئة التشغيل بأحدث إصدار بايثون 3.14
FROM python:3.14-alpine3.23

# تسطيب FFmpeg و Node.js (لحل شفرات يوتيوب) والاعتماديات الأساسية
RUN apk add --no-cache ffmpeg ca-certificates tzdata nodejs

# إعداد المستخدم والمجلدات
RUN addgroup -S anniegroup && adduser -S annieuser -G anniegroup
WORKDIR /app

# إعداد الـ Virtual Environment للبايثون
ENV VIRTUAL_ENV=/opt/venv
RUN python3 -m venv $VIRTUAL_ENV
ENV PATH="$VIRTUAL_ENV/bin:$PATH"

# تسطيب أحدث نسخة من yt-dlp (إصدارات 2026)
RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir "yt-dlp>=2026.04.01"

# نسخ الملف التنفيذي من مرحلة البناء
COPY --from=builder --chown=annieuser:anniegroup /build/annie-api /app/annie-api

# نسخ الكوكيز مباشرة من الريبو للسيرفر
COPY --chown=annieuser:anniegroup cookies.txt /app/cookies.txt

# تأمين الملفات
RUN chmod 500 /app/annie-api && chmod 400 /app/cookies.txt

ENV PORT=7860 \
    GIN_MODE=release \
    PYTHONUNBUFFERED=1

USER annieuser
EXPOSE 7860

# فحص الصحة لضمان إن السيرفر ميهنجش
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:7860/health || exit 1

ENTRYPOINT ["/app/annie-api"]
