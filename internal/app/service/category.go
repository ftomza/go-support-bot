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
	WelcomeNewUser   string `yaml:"WelcomeNewUser" json:"WelcomeNewUser"`
	AskForText       string `yaml:"AskForText" json:"AskForText"`
	TopicCreated     string `yaml:"TopicCreated" json:"TopicCreated"`
	OutOfHours       string `yaml:"OutOfHours" json:"OutOfHours"`
	ServerError      string `yaml:"ServerError" json:"ServerError"`
	CloseTopicButton string `yaml:"CloseTopicButton" json:"CloseTopicButton"`

	// НОВЫЕ ПОЛЯ ДЛЯ КЛИЕНТОВ
	TopicClosedByManager string `yaml:"TopicClosedByManager" json:"TopicClosedByManager"`
	TopicClosedByClient  string `yaml:"TopicClosedByClient" json:"TopicClosedByClient"`
	TopicAlreadyClosed   string `yaml:"TopicAlreadyClosed" json:"TopicAlreadyClosed"`
	PromptNewQuestions   string `yaml:"PromptNewQuestions" json:"PromptNewQuestions"`
	PromptReturn         string `yaml:"PromptReturn" json:"PromptReturn"`
	SelectTopic          string `yaml:"SelectTopic" json:"SelectTopic"`
	SelectSubtopic       string `yaml:"SelectSubtopic" json:"SelectSubtopic"`
	ButtonBack           string `yaml:"ButtonBack" json:"ButtonBack"`
	ButtonHome           string `yaml:"ButtonHome" json:"ButtonHome"`
	RateService          string `yaml:"RateService" json:"RateService"`
	RatingThanks         string `yaml:"RatingThanks" json:"RatingThanks"`

	// НОВЫЕ ПОЛЯ ДЛЯ МЕНЕДЖЕРОВ
	NotifyManagerNew         string `yaml:"NotifyManagerNew" json:"NotifyManagerNew"`
	NotifyTopicCreated       string `yaml:"NotifyTopicCreated" json:"NotifyTopicCreated"`
	NotifyTopicClosedClient  string `yaml:"NotifyTopicClosedClient" json:"NotifyTopicClosedClient"`
	NotifyTopicClosedManager string `yaml:"NotifyTopicClosedManager" json:"NotifyTopicClosedManager"`
	NotifyTopicRecreated     string `yaml:"NotifyTopicRecreated" json:"NotifyTopicRecreated"`
}

func GetDefaultMessages() YamlMessages {
	return YamlMessages{
		WelcomeNewUser:   "Привет! Как к тебе обращаться? Напиши свои имя и фамилию.",
		AskForText:       "Пожалуйста, используй текст.",
		TopicCreated:     "✅ Обращение зарегистрировано! Напишите ваш вопрос ниже.",
		OutOfHours:       "🌙 <b>Внимание: нерабочее время</b>\nМенеджеры этой линии сейчас отдыхают. Ваше сообщение сохранено и будет обработано в рабочие часы (<b>%s</b>).",
		ServerError:      "Ошибка сервера. Попробуйте позже.",
		CloseTopicButton: "❌ Завершить обращение",
		RateService:      "Пожалуйста, оцените качество решения вашего вопроса:",
		RatingThanks:     "Спасибо за вашу оценку! ⭐️",

		TopicClosedByManager: "✅ Менеджер завершил диалог. Спасибо за обращение!",
		TopicClosedByClient:  "✅ Диалог успешно завершен. Спасибо!",
		TopicAlreadyClosed:   "Обращение уже было закрыто.",
		PromptNewQuestions:   "Если у вас появятся новые вопросы, выберите тему ниже:",
		PromptReturn:         "С возвращением! Пожалуйста, выберите тему вашего нового обращения с помощью кнопок:",
		SelectTopic:          "Выберите тему обращения:",
		SelectSubtopic:       "Выберите подтему:",
		ButtonBack:           "🔙 Назад",
		ButtonHome:           "🏠 В начало",

		NotifyManagerNew:         "🚨 Новое обращение!\n\n<b>Клиент:</b> %s\n<b>Тема:</b> %s\n\n👉 <a href=\"%s\">Перейти в топик</a>",
		NotifyTopicCreated:       "🔄 <b>Обращение открыто</b>\nВыбрана тема: %s\nМенеджер: <a href=\"tg://user?id=%d\">Ассистент</a>",
		NotifyTopicClosedClient:  "❌ <b>Обращение завершено клиентом.</b>\nТема закрыта для новых сообщений.",
		NotifyTopicClosedManager: "✅ Топик закрыт. Обращение клиента переведено в статус решенных.",
		NotifyTopicRecreated:     "🔄 <b>Обращение пересоздано</b> (старый топик был удален)\nВыбрана тема: %s\nКлиент: %s",
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
	Image     *string              `yaml:"Image,omitempty" json:"Image,omitempty"`
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
			Image:     c.Image,
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
	if cfg.Messages.CloseTopicButton == "" {
		cfg.Messages.CloseTopicButton = defaults.CloseTopicButton
	}
	if cfg.Messages.RateService == "" {
		cfg.Messages.RateService = defaults.RateService
	}
	if cfg.Messages.RatingThanks == "" {
		cfg.Messages.RatingThanks = defaults.RatingThanks
	}

	if cfg.Messages.TopicClosedByManager == "" {
		cfg.Messages.TopicClosedByManager = defaults.TopicClosedByManager
	}
	if cfg.Messages.TopicClosedByClient == "" {
		cfg.Messages.TopicClosedByClient = defaults.TopicClosedByClient
	}
	if cfg.Messages.TopicAlreadyClosed == "" {
		cfg.Messages.TopicAlreadyClosed = defaults.TopicAlreadyClosed
	}
	if cfg.Messages.PromptNewQuestions == "" {
		cfg.Messages.PromptNewQuestions = defaults.PromptNewQuestions
	}
	if cfg.Messages.PromptReturn == "" {
		cfg.Messages.PromptReturn = defaults.PromptReturn
	}
	if cfg.Messages.SelectTopic == "" {
		cfg.Messages.SelectTopic = defaults.SelectTopic
	}
	if cfg.Messages.SelectSubtopic == "" {
		cfg.Messages.SelectSubtopic = defaults.SelectSubtopic
	}
	if cfg.Messages.ButtonBack == "" {
		cfg.Messages.ButtonBack = defaults.ButtonBack
	}
	if cfg.Messages.ButtonHome == "" {
		cfg.Messages.ButtonHome = defaults.ButtonHome
	}
	if cfg.Messages.NotifyManagerNew == "" {
		cfg.Messages.NotifyManagerNew = defaults.NotifyManagerNew
	}
	if cfg.Messages.NotifyTopicCreated == "" {
		cfg.Messages.NotifyTopicCreated = defaults.NotifyTopicCreated
	}
	if cfg.Messages.NotifyTopicClosedClient == "" {
		cfg.Messages.NotifyTopicClosedClient = defaults.NotifyTopicClosedClient
	}
	if cfg.Messages.NotifyTopicClosedManager == "" {
		cfg.Messages.NotifyTopicClosedManager = defaults.NotifyTopicClosedManager
	}
	if cfg.Messages.NotifyTopicRecreated == "" {
		cfg.Messages.NotifyTopicRecreated = defaults.NotifyTopicRecreated
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
			Image:      yt.Image, // Сохраняем переданный порядок
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

	// ЗАЩИТА: Если в старой БД нет новых полей, заполняем их дефолтными значениями
	defaults := GetDefaultMessages()
	if msgs.WelcomeNewUser == "" {
		msgs.WelcomeNewUser = defaults.WelcomeNewUser
	}
	if msgs.AskForText == "" {
		msgs.AskForText = defaults.AskForText
	}
	if msgs.TopicCreated == "" {
		msgs.TopicCreated = defaults.TopicCreated
	}
	if msgs.OutOfHours == "" {
		msgs.OutOfHours = defaults.OutOfHours
	}
	if msgs.ServerError == "" {
		msgs.ServerError = defaults.ServerError
	}
	if msgs.CloseTopicButton == "" {
		msgs.CloseTopicButton = defaults.CloseTopicButton
	}
	if msgs.RateService == "" {
		msgs.RateService = defaults.RateService
	}
	if msgs.RatingThanks == "" {
		msgs.RatingThanks = defaults.RatingThanks
	}

	// Добавьте это сразу после проверки CloseTopicButton
	if msgs.TopicClosedByManager == "" {
		msgs.TopicClosedByManager = defaults.TopicClosedByManager
	}
	if msgs.TopicClosedByClient == "" {
		msgs.TopicClosedByClient = defaults.TopicClosedByClient
	}
	if msgs.TopicAlreadyClosed == "" {
		msgs.TopicAlreadyClosed = defaults.TopicAlreadyClosed
	}
	if msgs.PromptNewQuestions == "" {
		msgs.PromptNewQuestions = defaults.PromptNewQuestions
	}
	if msgs.PromptReturn == "" {
		msgs.PromptReturn = defaults.PromptReturn
	}
	if msgs.SelectTopic == "" {
		msgs.SelectTopic = defaults.SelectTopic
	}
	if msgs.SelectSubtopic == "" {
		msgs.SelectSubtopic = defaults.SelectSubtopic
	}
	if msgs.ButtonBack == "" {
		msgs.ButtonBack = defaults.ButtonBack
	}
	if msgs.ButtonHome == "" {
		msgs.ButtonHome = defaults.ButtonHome
	}
	if msgs.NotifyManagerNew == "" {
		msgs.NotifyManagerNew = defaults.NotifyManagerNew
	}
	if msgs.NotifyTopicCreated == "" {
		msgs.NotifyTopicCreated = defaults.NotifyTopicCreated
	}
	if msgs.NotifyTopicClosedClient == "" {
		msgs.NotifyTopicClosedClient = defaults.NotifyTopicClosedClient
	}
	if msgs.NotifyTopicClosedManager == "" {
		msgs.NotifyTopicClosedManager = defaults.NotifyTopicClosedManager
	}
	if msgs.NotifyTopicRecreated == "" {
		msgs.NotifyTopicRecreated = defaults.NotifyTopicRecreated
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
		msgs, _ := s.GetMessages(ctx)
		currCat, err := s.GetCategoryByID(ctx, *parentID)
		if err == nil {
			var navRow []telego.InlineKeyboardButton

			if currCat.ParentID == nil {
				navRow = append(navRow, telego.InlineKeyboardButton{Text: msgs.ButtonBack, CallbackData: "cat_root"})
			} else {
				navRow = append(navRow, telego.InlineKeyboardButton{Text: msgs.ButtonBack, CallbackData: fmt.Sprintf("cat_%d", *currCat.ParentID)})
				navRow = append(navRow, telego.InlineKeyboardButton{Text: msgs.ButtonHome, CallbackData: "cat_root"})
			}
			keyboard = append(keyboard, navRow)
		} else {
			keyboard = append(keyboard, []telego.InlineKeyboardButton{
				{Text: msgs.ButtonHome, CallbackData: "cat_root"},
			})
		}
	}

	return &telego.InlineKeyboardMarkup{InlineKeyboard: keyboard}, nil
}

func (s *SupportService) GetCategoryByID(ctx context.Context, id int) (*datastruct.Category, error) {
	return s.repo.GetCategoryByID(ctx, id)
}
