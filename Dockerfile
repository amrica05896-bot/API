# STAGE 1: بناء تطبيق Go (The Builder)
FROM golang:1.26-alpine3.23 AS builder
WORKDIR /build
RUN apk add --no-cache git
COPY . .
RUN go mod tidy
# بناء التطبيق بأقصى درجات الضغط والتحسين للسرعة
RUN go build -trimpath -ldflags="-s -w -extldflags '-static'" -o annie-api main.go

# STAGE 2: بيئة التشغيل النهائية (The Speed-of-Light Runtime)
FROM python:3.14-alpine3.23

# 1. مكتبات السرعة الصاروخية
# - quickjs: أسرع بـ 100 مرة في الإقلاع من nodejs لفك تشفير يوتيوب!
# - aria2: أسرع محرك تحميل موازي في العالم (تحسباً لو هتحمل ميديا مستقبلاً)
# - py3-brotli & py3-certifi: لفك ضغط طلبات الويب بسرعة فائقة
RUN apk add --no-cache \
    ffmpeg \
    aria2 \
    quickjs \
    ca-certificates \
    tzdata \
    gcompat \
    libstdc++ \
    py3-brotli \
    py3-certifi \
    py3-websockets

# 2. إعداد المستخدم والبيئة
RUN addgroup -S anniegroup && adduser -S annieuser -G anniegroup
WORKDIR /app

# 3. إعداد البيئة الافتراضية لبايثون
ENV VIRTUAL_ENV=/opt/venv
RUN python3 -m venv $VIRTUAL_ENV
ENV PATH="$VIRTUAL_ENV/bin:/usr/bin:/usr/local/bin:$PATH"

# 4. تثبيت yt-dlp وعمل Compile مسبق (Pre-compile) للكود
# تحويل بايثون لـ Bytecode بيوفر وقت الـ Parsing في كل ريكويست بيتعمل!
RUN pip install --no-cache-dir --upgrade pip wheel && \
    pip install --no-cache-dir "yt-dlp>=2026.04.10" && \
    python3 -m compileall -q $VIRTUAL_ENV/lib/python3.14/site-packages/yt_dlp

# 5. نسخ ملفات التطبيق والكوكيز
COPY --from=builder --chown=annieuser:anniegroup /build/annie-api /app/annie-api
COPY --chown=annieuser:anniegroup cookies.txt /app/cookies.txt

# 6. تأمين الملفات مع إعطاء yt-dlp صلاحية تحديث الكوكيز (حل الإيرور 500)
RUN chmod 500 /app/annie-api && \
    chmod 600 /app/cookies.txt

# 7. إعدادات بيئية مخصصة للسرعة القصوى وتقليل استهلاك الرام
ENV PORT=7860 \
    GIN_MODE=release \
    PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    MALLOC_CONF="background_thread:true,metadata_thp:auto"

USER annieuser
EXPOSE 7860

ENTRYPOINT ["/app/annie-api"]
