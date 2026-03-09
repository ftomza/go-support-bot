/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package config

import (
	"log"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env      string         `yaml:"env" env-default:"local"`
	Telegram TelegramConfig `yaml:"telegram"`
	Database DatabaseConfig `yaml:"database"`
	Server   ServerConfig   `yaml:"server"`
	LLM      LLMConfig      `yaml:"llm"`
}

type LLMConfig struct {
	EnableTranslate bool   `yaml:"enable_translate" env:"ENABLE_TRANSLATE" env-default:"false"`
	GeminiAPIKey    string `yaml:"gemini_api_key" env:"GEMINI_API_KEY" env-required:"true"`
	ManagerLang     string `yaml:"manager_lang" env:"MANAGER_LANG" env-default:"ru"`
}

type TelegramConfig struct {
	Token          string  `yaml:"token" env:"TELEGRAM_TOKEN" env-required:"true"`
	SupportGroupID int64   `yaml:"support_group_id" env:"SUPPORT_GROUP_ID" env-required:"true"`
	UseWebhooks    bool    `yaml:"use_webhooks" env:"USE_WEBHOOKS" env-default:"false"`
	DeveloperIDs   []int64 `yaml:"developer_ids" env:"DEVELOPER_IDS"`
	WebhookURL     string  `yaml:"webhook_url" env:"WEBHOOK_URL"`
	MiniAppURL     string  `yaml:"mini_app_url" env:"MINI_APP_URL"`
}

type DatabaseConfig struct {
	URL string `yaml:"url" env:"DATABASE_URL" env-required:"true"`
}

type ServerConfig struct {
	Port     string `yaml:"port" env:"PORT" env-default:"8081"`
	CertFile string `yaml:"cert_file" env:"CERT_FILE"`
	KeyFile  string `yaml:"key_file" env:"KEY_FILE"`
}

// MustLoad читает конфиг и падает с fatal, если что-то идет не так (Fail-Fast подход)
func MustLoad(configPath string) *Config {
	// Проверяем, существует ли файл
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("Config file does not exist: %s", configPath)
	}

	var cfg Config
	// Читаем файл и переменные окружения
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("Cannot read config: %s", err)
	}

	return &cfg
}
