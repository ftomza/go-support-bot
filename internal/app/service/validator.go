package service

import (
	"fmt"
	"regexp"
	"strings"
)

// tgHTMLRegex ищет теги <tag>, </tag>
var tgHTMLRegex = regexp.MustCompile(`</?([a-zA-Z\-]+)[^>]*>`)

// ValidateTelegramHTML проверяет одну строку на корректность тегов
func ValidateTelegramHTML(text string) error {
	if text == "" || (!strings.Contains(text, "<") && !strings.Contains(text, ">")) {
		return nil
	}

	matches := tgHTMLRegex.FindAllStringSubmatch(text, -1)
	var stack []string

	allowedTags := map[string]bool{
		"b": true, "strong": true, "i": true, "em": true, "u": true, "ins": true,
		"s": true, "strike": true, "del": true, "a": true, "code": true, "pre": true,
		"tg-spoiler": true, "blockquote": true, "tg-emoji": true,
	}

	for _, match := range matches {
		fullTag := match[0]
		tagName := strings.ToLower(match[1])
		isClosing := strings.HasPrefix(fullTag, "</")

		if !allowedTags[tagName] {
			return fmt.Errorf("неподдерживаемый тег <%s>", tagName)
		}

		if isClosing {
			if len(stack) == 0 {
				return fmt.Errorf("найден закрывающий тег </%s> без открывающего", tagName)
			}
			last := stack[len(stack)-1]
			if last != tagName {
				return fmt.Errorf("нарушена вложенность: ожидалось </%s>, а найдено </%s>", last, tagName)
			}
			stack = stack[:len(stack)-1]
		} else {
			stack = append(stack, tagName)
		}
	}

	if len(stack) > 0 {
		return fmt.Errorf("не закрыт тег <%s>", stack[len(stack)-1])
	}

	aTags := regexp.MustCompile(`(?i)<a\s+[^>]*>`).FindAllString(text, -1)
	for _, a := range aTags {
		if !strings.Contains(strings.ToLower(a), "href=") {
			return fmt.Errorf("в теге <a> отсутствует атрибут href")
		}
		if !strings.Contains(a, `"`) && !strings.Contains(a, `'`) {
			return fmt.Errorf("атрибут href в теге <a> должен быть в кавычках")
		}
	}

	return nil
}

// ValidateYamlConfig проверяет всю конфигурацию (которая прилетает из WebApp)
func ValidateYamlConfig(cfg *YamlConfig) error {
	// 1. Проверяем главное сообщение
	if err := ValidateTelegramHTML(cfg.Text); err != nil {
		return fmt.Errorf("главное сообщение: %v", err)
	}

	// 2. Проверяем системные тексты
	msgs := map[string]string{
		"WelcomeNewUser":           cfg.Messages.WelcomeNewUser,
		"AskForText":               cfg.Messages.AskForText,
		"SelectTopic":              cfg.Messages.SelectTopic,
		"SelectSubtopic":           cfg.Messages.SelectSubtopic,
		"TopicCreated":             cfg.Messages.TopicCreated,
		"OutOfHours":               cfg.Messages.OutOfHours,
		"RateService":              cfg.Messages.RateService,
		"RatingThanks":             cfg.Messages.RatingThanks,
		"TopicClosedByManager":     cfg.Messages.TopicClosedByManager,
		"TopicClosedByClient":      cfg.Messages.TopicClosedByClient,
		"PromptNewQuestions":       cfg.Messages.PromptNewQuestions,
		"PromptReturn":             cfg.Messages.PromptReturn,
		"TopicAlreadyClosed":       cfg.Messages.TopicAlreadyClosed,
		"CloseTopicButton":         cfg.Messages.CloseTopicButton,
		"ButtonBack":               cfg.Messages.ButtonBack,
		"ButtonHome":               cfg.Messages.ButtonHome,
		"NotifyManagerNew":         cfg.Messages.NotifyManagerNew,
		"NotifyTopicCreated":       cfg.Messages.NotifyTopicCreated,
		"NotifyTopicClosedClient":  cfg.Messages.NotifyTopicClosedClient,
		"NotifyTopicClosedManager": cfg.Messages.NotifyTopicClosedManager,
		"NotifyTopicRecreated":     cfg.Messages.NotifyTopicRecreated,
		"ServerError":              cfg.Messages.ServerError,
		"AntiSpamWarning":          cfg.Messages.AntiSpamWarning,
	}

	for key, text := range msgs {
		if err := ValidateTelegramHTML(text); err != nil {
			return fmt.Errorf("ошибка в тексте [%s]: %v", key, err)
		}
	}

	// 3. Рекурсивно проверяем темы
	return validateYamlThemes(cfg.Themes)
}

func validateYamlThemes(themes map[string]YamlTheme) error {
	for name, theme := range themes {
		if err := ValidateTelegramHTML(theme.Text); err != nil {
			return fmt.Errorf("ошибка в категории [%s]: %v", name, err)
		}
		if theme.SubTheme != nil {
			if err := validateYamlThemes(theme.SubTheme); err != nil {
				return err
			}
		}
	}
	return nil
}
