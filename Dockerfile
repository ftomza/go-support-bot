# ==========================================
# ЭТАП 1: Собираем фронтенд (React)
# ==========================================
FROM node:25-alpine AS frontend-builder
WORKDIR /app/webapp
# Копируем конфиги и ставим зависимости Node.js
COPY webapp/package*.json ./
RUN npm install
# Копируем исходники фронта и билдим
COPY webapp/ ./
RUN npm run build

# ==========================================
# ЭТАП 2: Собираем бэкенд (Go)
# ==========================================
FROM golang:1.25-alpine AS backend-builder
WORKDIR /app
# Кэшируем Go модули
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# ВАЖНО: Забираем собранный React из первого этапа в папку web/dist
COPY --from=frontend-builder /app/web/dist ./web/dist
# Собираем один единственный бинарник (CGO_ENABLED=0 для минимального размера)
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/bot ./cmd/go-support-bot/main.go

# ==========================================
# ЭТАП 3: Финальный образ (максимально легкий)
# ==========================================
FROM alpine:latest
WORKDIR /app
# Копируем только сам бинарник, миграции и конфиг
COPY --from=backend-builder /app/bin/bot ./bot
COPY config/prod.yaml ./config/prod.yaml

# Указываем путь к конфигу
ENV CONFIG_PATH="config/prod.yaml"

# Запускаем нашего бота
CMD ["./bot"]