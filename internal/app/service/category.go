package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"go-support-bot/internal/app/datastruct"

	"github.com/mymmrac/telego"
	"gopkg.in/yaml.v3"
)

type YamlMessages struct {
	WelcomeNewUser string `yaml:"WelcomeNewUser" json:"WelcomeNewUser"`
	AskForText     string `yaml:"AskForText" json:"AskForText"`
	TopicCreated   string `yaml:"TopicCreated" json:"TopicCreated"`
	OutOfHours     string `yaml:"OutOfHours" json:"OutOfHours"`
	ServerError    string `yaml:"ServerError" json:"ServerError"`
}

func GetDefaultMessages() YamlMessages {
	return YamlMessages{
		WelcomeNewUser: "Привет! Как к тебе обращаться? Напиши свои имя и фамилию.",
		AskForText:     "Пожалуйста, используй текст.",
		TopicCreated:   "✅ Обращение зарегистрировано! Напишите ваш вопрос ниже.",
		OutOfHours:     "🌙 <b>Внимание: нерабочее время</b>\nМенеджеры этой линии сейчас отдыхают. Ваше сообщение сохранено и будет обработано в рабочие часы (<b>%s</b>).",
		ServerError:    "Ошибка сервера. Попробуйте позже.",
	}
}

type YamlConfig struct {
	Text     string               `yaml:"Text" json:"Text"`
	Messages YamlMessages         `yaml:"Messages" json:"Messages"`
	Themes   map[string]YamlTheme `yaml:"Themes" json:"Themes"`
}

type YamlTheme struct {
	Order     int                  `yaml:"Order" json:"Order"` // <--- ДОБАВЛЕНО
	Text      string               `yaml:"Text,omitempty" json:"Text,omitempty"`
	Manager   *int64               `yaml:"Manager,omitempty" json:"Manager,omitempty"`
	WorkHours *string              `yaml:"WorkHours,omitempty" json:"WorkHours,omitempty"`
	Timezone  *string              `yaml:"Timezone,omitempty" json:"Timezone,omitempty"`
	SubTheme  map[string]YamlTheme `yaml:"SubTheme,omitempty" json:"SubTheme,omitempty"`
}

func (s *SupportService) ExportConfig(ctx context.Context) (*YamlConfig, error) {
	prompt, _ := s.repo.GetMainPrompt(ctx)
	msgs, _ := s.GetMessages(ctx)

	cfg := &YamlConfig{
		Text:     prompt,
		Messages: msgs,
		Themes:   make(map[string]YamlTheme),
	}

	cats, err := s.repo.GetAllCategoriesFull(ctx)
	if err != nil {
		return nil, err
	}

	childrenMap := make(map[int][]datastruct.Category)
	var roots []datastruct.Category
	for _, c := range cats {
		if c.ParentID == nil {
			roots = append(roots, c)
		} else {
			childrenMap[*c.ParentID] = append(childrenMap[*c.ParentID], c)
		}
	}

	var buildTheme func(c datastruct.Category, order int) YamlTheme
	buildTheme = func(c datastruct.Category, order int) YamlTheme {
		yt := YamlTheme{
			Order:     order, // Раздаем порядковые номера при экспорте
			Text:      c.PromptText,
			Manager:   c.ManagerID,
			WorkHours: c.WorkHours,
			Timezone:  c.Timezone,
		}
		children := childrenMap[c.ID]
		if len(children) > 0 {
			yt.SubTheme = make(map[string]YamlTheme)
			for i, child := range children {
				yt.SubTheme[child.Name] = buildTheme(child, i)
			}
		}
		return yt
	}

	for i, r := range roots {
		cfg.Themes[r.Name] = buildTheme(r, i)
	}

	return cfg, nil
}

func (s *SupportService) ImportConfig(ctx context.Context, cfg *YamlConfig) error {
	defaults := GetDefaultMessages()
	if cfg.Messages.WelcomeNewUser == "" {
		cfg.Messages.WelcomeNewUser = defaults.WelcomeNewUser
	}
	if cfg.Messages.AskForText == "" {
		cfg.Messages.AskForText = defaults.AskForText
	}
	if cfg.Messages.TopicCreated == "" {
		cfg.Messages.TopicCreated = defaults.TopicCreated
	}
	if cfg.Messages.OutOfHours == "" {
		cfg.Messages.OutOfHours = defaults.OutOfHours
	}
	if cfg.Messages.ServerError == "" {
		cfg.Messages.ServerError = defaults.ServerError
	}

	msgBytes, _ := json.Marshal(cfg.Messages)

	var buildNode func(name string, yt YamlTheme) *datastruct.CategoryNode
	buildNode = func(name string, yt YamlTheme) *datastruct.CategoryNode {
		node := &datastruct.CategoryNode{
			Name:       name,
			PromptText: yt.Text,
			ManagerID:  yt.Manager,
			WorkHours:  yt.WorkHours,
			Timezone:   yt.Timezone,
			Order:      yt.Order, // Сохраняем переданный порядок
		}
		for k, v := range yt.SubTheme {
			node.Children = append(node.Children, buildNode(k, v))
		}
		// Сортируем подкатегории перед вставкой
		sort.Slice(node.Children, func(i, j int) bool {
			return node.Children[i].Order < node.Children[j].Order
		})
		return node
	}

	var roots []*datastruct.CategoryNode
	for k, v := range cfg.Themes {
		roots = append(roots, buildNode(k, v))
	}

	// Сортируем корни перед вставкой
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].Order < roots[j].Order
	})

	return s.repo.ReplaceCategoriesTree(ctx, cfg.Text, msgBytes, roots)
}

func (s *SupportService) LoadCategoriesFromYAML(ctx context.Context, data []byte) error {
	var cfg YamlConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("yaml parse error: %w", err)
	}
	return s.ImportConfig(ctx, &cfg)
}

func (s *SupportService) ExportCategoriesToYAML(ctx context.Context) ([]byte, error) {
	cfg, err := s.ExportConfig(ctx)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(cfg)
}

func (s *SupportService) GetMessages(ctx context.Context) (YamlMessages, error) {
	val, err := s.repo.GetSetting(ctx, "messages")
	if err != nil || val == "" {
		return GetDefaultMessages(), nil
	}
	var msgs YamlMessages
	if err := json.Unmarshal([]byte(val), &msgs); err != nil {
		return GetDefaultMessages(), nil
	}
	return msgs, nil
}

func (s *SupportService) GetMainPrompt(ctx context.Context) (string, error) {
	return s.repo.GetMainPrompt(ctx)
}

func (s *SupportService) GetCategoriesKeyboard(ctx context.Context, parentID *int) (*telego.InlineKeyboardMarkup, error) {
	categories, err := s.repo.GetCategoriesByParent(ctx, parentID)
	if err != nil {
		return nil, err
	}

	var keyboard [][]telego.InlineKeyboardButton
	for _, c := range categories {
		keyboard = append(keyboard, []telego.InlineKeyboardButton{
			{Text: c.Name, CallbackData: fmt.Sprintf("cat_%d", c.ID)},
		})
	}

	// Если мы находимся внутри подменю, добавляем кнопки навигации
	if parentID != nil {
		currCat, err := s.GetCategoryByID(ctx, *parentID)
		if err == nil {
			var navRow []telego.InlineKeyboardButton

			if currCat.ParentID == nil {
				// Мы на первом уровне вложенности -> возврат в самый корень
				navRow = append(navRow, telego.InlineKeyboardButton{Text: "🔙 Back", CallbackData: "cat_root"})
			} else {
				// Мы глубже 1-го уровня -> возврат на папку выше + кнопка в корень
				navRow = append(navRow, telego.InlineKeyboardButton{Text: "🔙 Back", CallbackData: fmt.Sprintf("cat_%d", *currCat.ParentID)})
				navRow = append(navRow, telego.InlineKeyboardButton{Text: "🏠 To Begin", CallbackData: "cat_root"})
			}

			keyboard = append(keyboard, navRow)
		} else {
			// Обычный фолбэк на случай непредвиденной ошибки БД
			keyboard = append(keyboard, []telego.InlineKeyboardButton{
				{Text: "🔙 В начало", CallbackData: "cat_root"},
			})
		}
	}

	return &telego.InlineKeyboardMarkup{InlineKeyboard: keyboard}, nil
}

func (s *SupportService) GetCategoryByID(ctx context.Context, id int) (*datastruct.Category, error) {
	return s.repo.GetCategoryByID(ctx, id)
}
