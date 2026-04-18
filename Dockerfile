# STAGE 1: بناء تطبيق Go (The Builder)
FROM golang:1.26-alpine3.23 AS builder
WORKDIR /build
RUN apk add --no-cache git
COPY . .
RUN go mod tidy
RUN go build -trimpath -ldflags="-s -w -extldflags '-static'" -o annie-api main.go

# STAGE 2: بيئة التشغيل النهائية (The Runtime)
FROM python:3.14-alpine3.23

# 1. تثبيت الاعتماديات ومكتبة التوافق gcompat اللي حلت مشكلة الـ Node في Alpine
RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    tzdata \
    nodejs \
    gcompat \
    libstdc++

# 2. إعداد المستخدم والبيئة
RUN addgroup -S anniegroup && adduser -S annieuser -G anniegroup
WORKDIR /app

# 3. إعداد البيئة الافتراضية لبايثون
ENV VIRTUAL_ENV=/opt/venv
RUN python3 -m venv $VIRTUAL_ENV
ENV PATH="$VIRTUAL_ENV/bin:/usr/bin:/usr/local/bin:$PATH"

# 4. ربط Node.js داخل الـ venv وفي المسارات الأساسية (Symlinks)
# الخطوة دي بتجبر yt-dlp إنه يشوف المحرك وما يقولش "JS runtimes: none"
RUN ln -sf /usr/bin/node /opt/venv/bin/node && \
    ln -sf /usr/bin/node /usr/bin/nodejs

# 5. تثبيت yt-dlp بأحدث نسخة
RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir "yt-dlp>=2026.04.10"

# 6. نسخ ملفات التطبيق من المرحلة الأولى والكوكيز
COPY --from=builder --chown=annieuser:anniegroup /build/annie-api /app/annie-api
COPY --chown=annieuser:anniegroup cookies.txt /app/cookies.txt

# تأمين الملفات
RUN chmod 500 /app/annie-api && chmod 400 /app/cookies.txt

# الإعدادات البيئية للسيرفر الـ 10 كور
ENV PORT=7860 \
    GIN_MODE=release \
    PYTHONUNBUFFERED=1

USER annieuser
EXPOSE 7860

ENTRYPOINT ["/app/annie-api"]
