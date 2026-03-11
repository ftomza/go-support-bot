# 🤖 Enterprise Telegram Support Bot

[![Go-report](https://img.shields.io/badge/go--report-A+-blue.svg)](https://goreportcard.com/report/github.com/ftomza/go-support-bot)
[![License](https://img.shields.io/badge/license-Apache-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)

Современный, производительный и многоязычный бот технической поддержки для Telegram. Построен на **Go** с использованием чистого API, оснащен встроенной **React WebApp** панелью управления и интегрирован с **Google Gemini AI** для бесшовного перевода сообщений между клиентами и менеджерами.

## ✨ Ключевые возможности

* **🌐 Бесшовный AI Переводчик (Gemini 2.5 Flash):** Бот автоматически определяет язык клиента. Если клиент пишет на испанском, а менеджер говорит по-русски, бот переводит сообщения "на лету" в обе стороны.
* **🖥️ WebApp Панель Администратора:** Полноценный визуальный редактор прямо внутри Telegram (React + Tailwind CSS). Позволяет настраивать дерево категорий, назначать менеджеров и менять системные тексты без редактирования кода.
* **🕒 Умный график работы:** Настройка рабочих часов (например, `09:00-18:00`) для каждой категории отдельно. При обращении в нерабочее время клиент получает вежливую отбивку.
* **📦 Автоматические миграции (Goose):** База данных обновляется автоматически при запуске приложения. SQL-миграции "запекаются" в бинарный файл с помощью `go:embed`.
* **🛡️ Защита от сбоев (Panic Recovery):** Критические ошибки не "убивают" бота. Перехватчик шлет `stack trace` разработчикам в личку и продолжает работу.
* **⚡ Webhooks & SSL:** Готов к высоким нагрузкам. Поддерживает работу через webhook по HTTPS (с использованием собственных сертификатов, например, Cloudflare Origin CA).
* **👨‍💻 Режим тестирования:** Команда `/client_mode` позволяет администраторам тестировать клиентскую воронку и меню, не теряя при этом прав менеджера в группе.

## 🛠 Технологический стек

* **Backend:** Go (Golang) 1.25+, `telego` (Telegram Bot API), `net/http`
* **Frontend (Admin Panel):** React, Vite, Tailwind CSS, Telegram WebApp API
* **Database:** PostgreSQL, `pgx` (driver), `goose` (migrations)
* **AI Integration:** Google Generative AI SDK (Gemini)
* **Deployment:** Docker, Docker Compose, GitHub Actions (CI/CD)

## 📁 Структура проекта

```text
├── cmd/go-support-bot/   # Точка входа в приложение (main.go)
├── config/               # Конфигурационные файлы (example.yaml)
├── internal/
│   └── app/
│       ├── clients/      # Интеграции со сторонними API (Telegram, Gemini)
│       ├── config/       # Парсинг YAML конфигурации
│       ├── datastruct/   # Основные структуры данных
│       ├── endpoints/    # Обработчики Telegram и HTTP API
│       ├── repository/   # Слой работы с БД (PostgreSQL)
│       └── service/      # Бизнес-логика бота
├── migration/            # SQL скрипты миграций БД (встраиваются через go:embed)
├── web/                  # Скомпилированный React-фронтенд (встраивается через go:embed)
├── webapp/               # Исходный код React WebApp (Vite)
├── Dockerfile            # Multi-stage сборка проекта
└── docker-compose.yml    # Инфраструктура для продакшена
```

## 🚀 Локальный запуск (Development)

### 1. Подготовка окружения

Убедитесь, что у вас установлены:

* [Go](https://go.dev/doc/install) (1.25+)
* [Node.js](https://nodejs.org/) (для работы с фронтендом)
* [Docker](https://www.docker.com/) (для локальной БД)
* [ngrok](https://ngrok.com/) (опционально, для проброса вебхуков)

### 2. Настройка конфигурации

Создайте файл `config/local.yaml` (возьмите за основу `config/example.yaml`) и заполните данные:

* `telegram.token` — Токен вашего бота (от @BotFather).
* `telegram.support_group_id` — ID группы, куда будут падать обращения.
* `telegram.developer_ids` — Массив ID разработчиков (туда будут приходить ошибки).
* `llm.gemini_api_key` — Ключ от Google AI Studio.

### 3. Запуск проекта

Поднимите базу данных:

```bash
docker-compose up db -d

```

Запустите фронтенд в режиме разработки (в отдельном терминале):

```bash
cd webapp
npm install
npm run dev

```

Запустите бэкенд:

```bash
go run cmd/go-support-bot/main.go --config config/local.yaml

```

*(Бот автоматически применит SQL миграции к базе при первом запуске).*

## 🌍 Развертывание (Production)

Проект собирается в **один легковесный Docker-контейнер** (Multi-stage build). Фронтенд и SQL-миграции компилируются и "запекаются" прямо в бинарник Go.

### Настройка SSL (Cloudflare Origin CA)

Если вы используете строгий режим Cloudflare (`Full Strict`):

1. Выпустите сертификаты в панели Cloudflare.
2. Создайте папку `certs` рядом с `docker-compose.yml`.
3. Положите туда файлы `cert.pem` и `key.pem`.
4. В `prod.yaml` укажите пути: `cert_file: "/app/certs/cert.pem"`, `key_file: "/app/certs/key.pem"`.

### Деплой на VPS

1. Склонируйте репозиторий на сервер.
2. Создайте файл `config/prod.yaml` с боевыми ключами.
3. Выполните сборку и запуск:

```bash
docker compose up -d --build

```

### CI/CD (GitHub Actions)

Проект содержит настроенный workflow `.github/workflows/deploy.yml`.
При пуше в ветку `main`, GitHub автоматически подключится к вашему серверу по SSH и обновит контейнеры.
*Необходимо настроить секреты репозитория: `HOST`, `USERNAME` и `SSH_KEY`.*

## 🔒 Безопасность

* Все запросы к API от WebApp валидируются с использованием `X-Telegram-Init-Data` и токена бота, что исключает подделку данных.
* Доступ к панели управления (команда `/admin`) разрешен только менеджерам из группы поддержки, а сама кнопка отправляется администратору в личные сообщения для приватности.

## 📝 Лицензия

Этот проект лицензирован по лицензии Apache 2.0. Подробнее см. в файле [LICENSE](https://www.google.com/search?q=LICENSE).

```