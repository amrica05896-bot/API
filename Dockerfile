# STAGE 1: البناء (The Builder)
FROM golang:1.26-alpine3.23 AS builder
WORKDIR /build
RUN apk add --no-cache git
COPY . .
RUN go mod tidy
RUN go build -trimpath -ldflags="-s -w -extldflags '-static'" -o annie-api main.go

# STAGE 2: بيئة التشغيل الصاروخية (The Pre-loaded Runtime)
FROM python:3.14-alpine3.23

# 1. تثبيت كل المحركات الممكنة لضمان عدم الفشل
RUN apk add --no-cache \
    ffmpeg \
    nodejs \
    aria2 \
    ca-certificates \
    tzdata \
    gcompat \
    libstdc++ \
    py3-brotli

# 2. إعداد المستخدم والمسارات الثابتة للكاش
RUN addgroup -S anniegroup && adduser -S annieuser -G anniegroup
WORKDIR /app

# تعريف مسارات الكاش كمتغيرات بيئة ثابتة (ده بيخلي السيرفر "حافظ" الملفات)
ENV XDG_CACHE_HOME=/app/.cache \
    XDG_CONFIG_HOME=/app/.config \
    PYTHONPYCACHEPREFIX=/app/.pycache

RUN mkdir -p /app/.cache /app/.config /app/.pycache && \
    chown -R annieuser:anniegroup /app

# 3. إعداد البيئة الافتراضية
ENV VIRTUAL_ENV=/opt/venv
RUN python3 -m venv $VIRTUAL_ENV
ENV PATH="$VIRTUAL_ENV/bin:$PATH"

# 4. تحميل yt-dlp وعمل "حقن" لكل المكونات عن بُعد أثناء البناء
# السطر ده بيخلي الدوكر ينزل سكريبتات فك التشفير (EJS) ويحفظها جوه الـ Image
RUN pip install --no-cache-dir --upgrade pip wheel && \
    pip install --no-cache-dir "yt-dlp>=2026.04.10" && \
    yt-dlp --remote-components ejs:github --version && \
    python3 -m compileall -q $VIRTUAL_ENV/lib/python3.14/site-packages/yt_dlp

# 5. نسخ الملفات ومنح الصلاحيات
COPY --from=builder --chown=annieuser:anniegroup /build/annie-api /app/annie-api
COPY --chown=annieuser:anniegroup cookies.txt /app/cookies.txt

# تأمين الملفات والسماح بتحديث الكوكيز والكاش
RUN chown -R annieuser:anniegroup /app && \
    chmod 500 /app/annie-api && \
    chmod 600 /app/cookies.txt

# 6. إعدادات الأداء العالي للسيرفرات الـ 16 كور
ENV PORT=7860 \
    GIN_MODE=release \
    PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=0 \
    MALLOC_CONF="background_thread:true,metadata_thp:auto"

USER annieuser
EXPOSE 7860

# الأمر ده بيعمل "تنشيط" للكاش أول ما السيرفر يفتح
ENTRYPOINT ["/app/annie-api"]
