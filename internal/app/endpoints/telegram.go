/*
 * Copyright © 2026-present Artem V. Zaborskiy <ftomza@yandex.ru>. All rights reserved.
 *
 * This source code is licensed under the Apache 2.0 license found in the LICENSE file in the root directory of this source tree.
 */

package endpoints

import (
	"context"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	"go-support-bot/internal/app/clients/telegram"
	"go-support-bot/internal/app/service"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

type TelegramEndpoints struct {
	svc        *service.SupportService
	devIDs     []int64
	miniAppURL string
}

func NewTelegramEndpoints(svc *service.SupportService, devIDs []int64, miniAppURL string) *TelegramEndpoints {
	return &TelegramEndpoints{
		svc:        svc,
		devIDs:     devIDs,
		miniAppURL: miniAppURL,
	}
}

// Предикат для загрузки CSV
func isYamlUpload() th.Predicate {
	return func(ctx context.Context, update telego.Update) bool {
		return update.Message != nil && update.Message.Document != nil &&
			update.Message.Caption == "/load_yaml"
	}
}

// Метод для безопасной отправки ошибки разработчикам
func (e *TelegramEndpoints) notifyDevelopers(bot *telego.Bot, text string) {
	// Лимит сообщения в Telegram — 4096 символов. Если stack trace огромный, обрезаем его.
	if len(text) > 4000 {
		text = text[:4000] + "...</pre>"
	}

	for _, id := range e.devIDs {
		_, _ = bot.SendMessage(context.Background(), &telego.SendMessageParams{
			ChatID:    tu.ID(id),
			Text:      text,
			ParseMode: telego.ModeHTML,
		})
	}
}

func (e *TelegramEndpoints) Register(bh *th.BotHandler) {
	bh.Use(func(botCtx *th.Context, update telego.Update) error {
		// 1. Defer для перехвата критических сбоев (panic)
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Printf("PANIC RECOVERED: %v\n%s", r, stack)

				errStr := fmt.Sprintf("🚨 <b>ПАНИКА В БОТЕ!</b>\n\n<b>Событие:</b> <code>%d</code>\n<b>Ошибка:</b> %v\n\n<pre>%s</pre>",
					update.UpdateID, r, html.EscapeString(stack))

				e.notifyDevelopers(botCtx.Bot(), errStr)
			}
		}()

		// 2. Выполняем следующий хендлер, передавая ему и контекст, и апдейт
		err := botCtx.Next(update)

		// 3. Если хендлер вернул обычную ошибку (не панику), тоже шлем её разработчикам
		if err != nil {
			errStr := fmt.Sprintf("⚠️ <b>Ошибка обработки (Update %d):</b>\n<pre>%v</pre>",
				update.UpdateID, html.EscapeString(err.Error()))
			e.notifyDevelopers(botCtx.Bot(), errStr)
		}

		return err
	})

	// КОМАНДА ДЛЯ ОТКРЫТИЯ WEBAPP ПАНЕЛИ
	bh.HandleMessage(func(botCtx *th.Context, message telego.Message) error {
		ctx := botCtx.Context()

		// Пытаемся отправить WebApp-кнопку В ЛИЧКУ администратору (message.From.ID)
		_, err := botCtx.Bot().SendMessage(ctx, &telego.SendMessageParams{
			ChatID:    tu.ID(message.From.ID), // <--- ИЗМЕНЕНО: Шлем в личку, а не в message.Chat.ID
			Text:      "⚙️ <b>Панель управления ботом</b>\nНажмите кнопку ниже, чтобы открыть визуальный редактор категорий и текстов.",
			ParseMode: telego.ModeHTML,
			ReplyMarkup: &telego.InlineKeyboardMarkup{
				InlineKeyboard: [][]telego.InlineKeyboardButton{
					{{
						Text:   "🖥 Открыть редактор",
						WebApp: &telego.WebAppInfo{URL: e.miniAppURL},
					}},
				},
			},
		})

		// Если бот выдал ошибку (например, админ никогда раньше не писал боту в ЛС и чат не активирован)
		if err != nil {
			_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(
				tu.ID(message.Chat.ID),
				"❌ Не могу отправить панель. Пожалуйста, перейдите в личные сообщения со мной, нажмите /start, а затем снова введите /admin в этой группе.",
			))
			return nil
		}

		// (Опционально) можно удалить сообщение с командой /admin из группы, чтобы не мусорить
		_ = botCtx.Bot().DeleteMessage(ctx, &telego.DeleteMessageParams{
			ChatID:    tu.ID(message.Chat.ID),
			MessageID: message.MessageID,
		})

		return nil
	}, th.CommandEqual("admin"), telegram.IsGroupChat())

	// КОМАНДА ДЛЯ ПЕРЕКЛЮЧЕНИЯ РЕЖИМА КЛИЕНТА (ТОЛЬКО ДЛЯ АДМИНОВ)
	bh.HandleMessage(func(botCtx *th.Context, message telego.Message) error {
		ctx := botCtx.Context()
		userID := message.From.ID

		// Проверяем, что юзер действительно является менеджером группы
		if !e.svc.IsManager(ctx, userID) {
			return nil
		}

		// Переключаем режим туда-обратно
		isEnabled := e.svc.ToggleTestMode(userID)

		msg := "👨‍💻 <b>Режим клиента ВЫКЛЮЧЕН</b>\nТеперь вы снова админ. Чтобы протестировать меню, включите режим обратно."
		if isEnabled {
			msg = "👤 <b>Режим клиента ВКЛЮЧЕН</b>\n\nТеперь бот будет общаться с вами так же, как с обычным пользователем.\n<i>(Напишите любое текстовое сообщение, чтобы вызвать меню)</i>"
		}

		_, err := botCtx.Bot().SendMessage(ctx, tu.Message(
			tu.ID(userID),
			msg,
		).WithParseMode(telego.ModeHTML))

		return err
	}, th.CommandEqual("client_mode"), telegram.IsPrivateChat())

	bh.HandleMessage(func(botCtx *th.Context, message telego.Message) error {
		ctx := botCtx.Context()

		list, err := e.svc.GetManagersList(ctx)
		if err != nil {
			log.Printf("Error getting managers list: %v", err)
			_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(
				tu.ID(message.Chat.ID),
				"❌ Ошибка при получении списка менеджеров.",
			))
			return nil
		}

		_, err = botCtx.Bot().SendMessage(ctx, tu.Message(
			tu.ID(message.Chat.ID),
			list,
		).WithParseMode(telego.ModeHTML))
		return err
	}, th.CommandEqual("managers"), telegram.IsGroupChat())

	// Команда /clearcache ТОЛЬКО для группы поддержки
	bh.HandleMessage(func(botCtx *th.Context, message telego.Message) error {
		ctx := botCtx.Context()
		e.svc.ClearCacheBotClient()

		_, err := botCtx.Bot().SendMessage(ctx, tu.Message(
			tu.ID(message.Chat.ID),
			"🔄 Кэш ролей пользователей успешно сброшен!",
		))
		return err
	}, th.CommandEqual("clearcache"), telegram.IsGroupChat())

	bh.HandleMessage(func(botCtx *th.Context, message telego.Message) error {
		ctx := botCtx.Context()

		yamlData, err := e.svc.ExportCategoriesToYAML(ctx)
		if err != nil {
			_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(tu.ID(message.Chat.ID), "❌ Ошибка выгрузки конфигурации."))
			return err
		}

		document := tu.Document(
			tu.ID(message.Chat.ID),
			tu.FileFromBytes(yamlData, "theme.yaml"),
		).WithCaption("⚙️ Текущая конфигурация бота. Отредактируйте и отправьте обратно с подписью /load_yaml")

		_, err = botCtx.Bot().SendDocument(ctx, document)
		return err
	}, th.CommandEqual("get_yaml"), telegram.IsGroupChat())

	// Команда /set_lang для принудительной смены языка клиента
	bh.HandleMessage(func(botCtx *th.Context, message telego.Message) error {
		ctx := botCtx.Context()

		// Проверяем, что команду вызвали внутри конкретного топика клиента
		if !message.IsTopicMessage {
			_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(
				tu.ID(message.Chat.ID),
				"❌ Эту команду нужно использовать внутри ветки (топика) клиента.",
			).WithMessageThreadID(message.MessageThreadID))
			return nil
		}

		// Разбиваем сообщение, чтобы достать аргумент (например, "es" из "/set_lang es")
		parts := strings.Fields(message.Text)
		if len(parts) < 2 {
			_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(
				tu.ID(message.Chat.ID),
				"❌ Пожалуйста, укажите код языка. Пример: <code>/set_lang es</code>",
			).WithParseMode(telego.ModeHTML).WithMessageThreadID(message.MessageThreadID))
			return nil
		}

		langCode := parts[1]
		topicID := message.MessageThreadID

		err := e.svc.SetCustomerLangByTopic(ctx, topicID, langCode)
		if err != nil {
			_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(
				tu.ID(message.Chat.ID),
				"❌ Ошибка: не удалось найти клиента для этого топика.",
			).WithMessageThreadID(message.MessageThreadID))
			return nil
		}

		_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(
			tu.ID(message.Chat.ID),
			fmt.Sprintf("✅ Язык клиента успешно изменен на <b>%s</b>.\nТеперь все его входящие и ваши исходящие сообщения будут переводиться с учетом этого языка.", html.EscapeString(langCode)),
		).WithParseMode(telego.ModeHTML).WithMessageThreadID(message.MessageThreadID))

		return nil
	}, th.CommandEqual("set_lang"), telegram.IsGroupChat())

	// 1. ЗАКРЫТИЕ ТОПИКА (Менеджер решил проблему)
	bh.HandleMessage(func(botCtx *th.Context, message telego.Message) error {
		ctx := botCtx.Context()
		topicID := message.MessageThreadID

		err := e.svc.CloseTopic(ctx, topicID)
		if err == nil {
			_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(
				tu.ID(message.Chat.ID),
				"✅ Топик закрыт. Обращение клиента переведено в статус решенных.",
			).WithReplyParameters(&telego.ReplyParameters{MessageID: message.MessageID}))

			// Находим клиента и убираем у него клавиатуру
			customerID, err := e.svc.GetCustomerID(ctx, topicID)
			if err == nil {
				_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(
					tu.ID(customerID),
					"✅ Менеджер завершил диалог. Спасибо за обращение!",
				).WithReplyMarkup(&telego.ReplyKeyboardRemove{RemoveKeyboard: true}))

				// Сразу предлагаем меню для новых вопросов
				kb, _ := e.svc.GetCategoriesKeyboard(ctx, nil)
				prompt, _ := e.svc.GetMainPrompt(ctx)
				if prompt == "" {
					prompt = "Если у вас появятся новые вопросы, выберите тему ниже:"
				}
				_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(
					tu.ID(customerID),
					prompt,
				).WithReplyMarkup(kb).WithParseMode(telego.ModeHTML))
			}
		}
		return err
	}, telegram.IsGroupChat(), telegram.IsTopicClosed())

	// 2. ЗАГРУЗКА CSV КОНФИГА
	bh.HandleMessage(func(botCtx *th.Context, message telego.Message) error {
		ctx := botCtx.Context()
		doc := message.Document

		file, err := botCtx.Bot().GetFile(ctx, &telego.GetFileParams{FileID: doc.FileID})
		if err != nil {
			return err
		}

		// Скачиваем файл напрямую через Telegram API
		fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botCtx.Bot().Token(), file.FilePath)
		resp, err := http.Get(fileURL)
		if err != nil || resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to download file")
		}
		defer resp.Body.Close()

		b, _ := io.ReadAll(resp.Body)

		if err := e.svc.LoadCategoriesFromYAML(ctx, b); err != nil {
			_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(tu.ID(message.Chat.ID), "❌ Ошибка парсинга YAML!"))
			return err
		}

		_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(tu.ID(message.Chat.ID), "✅ Темы и менеджеры успешно обновлены!"))
		return nil
	}, telegram.IsGroupChat(), isYamlUpload())

	// 4. НАЖАТИЕ НА КНОПКУ ТЕМЫ (CallbackQuery)
	bh.HandleCallbackQuery(func(botCtx *th.Context, query telego.CallbackQuery) error {
		ctx := botCtx.Context()
		customerID := query.From.ID

		_ = botCtx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})

		// Кнопка "В начало"
		if query.Data == "cat_root" {
			kb, _ := e.svc.GetCategoriesKeyboard(ctx, nil)
			prompt, _ := e.svc.GetMainPrompt(ctx)
			if prompt == "" {
				prompt = "Выберите тему обращения:"
			}

			// Удаляем старое сообщение
			_ = botCtx.Bot().DeleteMessage(ctx, &telego.DeleteMessageParams{
				ChatID:    tu.ID(customerID),
				MessageID: query.Message.GetMessageID(),
			})

			// Присылаем новое главное меню
			_, _ = botCtx.Bot().SendMessage(ctx, &telego.SendMessageParams{
				ChatID:      tu.ID(customerID),
				Text:        prompt,
				ReplyMarkup: kb,
				ParseMode:   telego.ModeHTML,
			})
			return nil
		}

		if !strings.HasPrefix(query.Data, "cat_") {
			return nil
		}

		categoryID, _ := strconv.Atoi(strings.TrimPrefix(query.Data, "cat_"))
		category, err := e.svc.GetCategoryByID(ctx, categoryID)
		if err != nil {
			return nil
		}

		// Сценарий А: Это ПАПКА (промежуточная тема)
		if category.ManagerID == nil {
			kb, _ := e.svc.GetCategoriesKeyboard(ctx, &category.ID)
			prompt := category.PromptText
			if prompt == "" {
				prompt = "Выберите подтему:"
			}

			// Удаляем старое меню
			_ = botCtx.Bot().DeleteMessage(ctx, &telego.DeleteMessageParams{
				ChatID:    tu.ID(customerID),
				MessageID: query.Message.GetMessageID(),
			})

			// Если есть картинка — шлем SendPhoto
			if category.Image != nil && *category.Image != "" {
				_, _ = botCtx.Bot().SendPhoto(ctx, &telego.SendPhotoParams{
					ChatID:      tu.ID(customerID),
					Photo:       tu.FileFromURL(*category.Image), // Загружаем по URL
					Caption:     prompt,
					ParseMode:   telego.ModeHTML,
					ReplyMarkup: kb,
				})
			} else {
				// Иначе обычный текст
				_, _ = botCtx.Bot().SendMessage(ctx, &telego.SendMessageParams{
					ChatID:      tu.ID(customerID),
					Text:        prompt,
					ParseMode:   telego.ModeHTML,
					ReplyMarkup: kb,
				})
			}
			return nil
		}

		// Сценарий Б: Это ФИНАЛЬНАЯ ТЕМА (лист) с назначенным менеджером
		session, exists := e.svc.GetSession(customerID)
		fullName := query.From.FirstName
		if exists && session.FullName != "" {
			fullName = session.FullName
		}

		msgs, _ := e.svc.GetMessages(ctx)
		err = e.svc.CreateOrReopenTopic(ctx, customerID, query.From.Username, fullName, categoryID, query.From.LanguageCode)
		if err != nil {
			_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(tu.ID(customerID), msgs.ServerError).WithParseMode(telego.ModeHTML))
			return nil
		}

		e.svc.ClearSession(customerID)

		// Удаляем инлайн-меню
		_ = botCtx.Bot().DeleteMessage(ctx, &telego.DeleteMessageParams{
			ChatID:    tu.ID(customerID),
			MessageID: query.Message.GetMessageID(),
		})

		// Создаем постоянную Reply-кнопку для клиента
		kb := tu.Keyboard(
			tu.KeyboardRow(tu.KeyboardButton(msgs.CloseTopicButton)),
		).WithResizeKeyboard()

		// Отправляем уведомление с кнопкой
		_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(tu.ID(customerID), msgs.TopicCreated).
			WithParseMode(telego.ModeHTML).
			WithReplyMarkup(kb))

		return nil
	}, e.svc.IsCustomer())

	// 3. ТЕКСТ ОТ КЛИЕНТА
	bh.HandleMessage(func(botCtx *th.Context, message telego.Message) error {
		ctx := botCtx.Context()
		customerID := message.From.ID

		// Проверяем, есть ли топик в базе
		topic, err := e.svc.GetCustomerTopic(ctx, customerID)

		// СЦЕНАРИЙ А: Топик ЕСТЬ и он ОТКРЫТ (просто пересылаем вопрос)
		if err == nil && !topic.IsClosed {
			msgs, _ := e.svc.GetMessages(ctx)

			// Проверяем, не нажал ли клиент кнопку закрытия
			if message.Text == msgs.CloseTopicButton {
				_ = e.svc.CloseTopicByClient(ctx, customerID)

				// Убираем клавиатуру и благодарим
				_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(
					tu.ID(customerID),
					"✅ Диалог успешно завершен. Спасибо!",
				).WithReplyMarkup(&telego.ReplyKeyboardRemove{RemoveKeyboard: true}))

				// Сразу выдаем главное меню
				catKb, _ := e.svc.GetCategoriesKeyboard(ctx, nil)
				prompt, _ := e.svc.GetMainPrompt(ctx)
				if prompt == "" {
					prompt = "Вы можете открыть новое обращение, выбрав тему ниже:"
				}
				_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(
					tu.ID(customerID),
					prompt,
				).WithReplyMarkup(catKb).WithParseMode(telego.ModeHTML))

				return nil
			}

			// Иначе просто пересылаем сообщение менеджеру
			return e.svc.HandleCustomerMessage(ctx, &message)
		}

		// СЦЕНАРИЙ Б: Топик ЕСТЬ, но он ЗАКРЫТ (клиент пишет новое обращение спустя время)
		if err == nil && topic.IsClosed {
			msgs, _ := e.svc.GetMessages(ctx)

			// Защита: если клиент случайно нажал старую кнопку, просто убираем её
			if message.Text == msgs.CloseTopicButton {
				_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(
					tu.ID(customerID),
					"Обращение уже было закрыто.",
				).WithReplyMarkup(&telego.ReplyKeyboardRemove{RemoveKeyboard: true}))
			}

			kb, _ := e.svc.GetCategoriesKeyboard(ctx, nil)
			prompt, _ := e.svc.GetMainPrompt(ctx)
			if prompt == "" {
				prompt = "С возвращением! Пожалуйста, выберите тему вашего нового обращения с помощью кнопок:"
			}
			_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(
				tu.ID(message.Chat.ID),
				prompt,
			).WithReplyMarkup(kb).WithParseMode(telego.ModeHTML))
			return nil
		}

		// СЦЕНАРИЙ В: Топика НЕТ ВООБЩЕ (новичок)
		session, exists := e.svc.GetSession(customerID)
		msgs, _ := e.svc.GetMessages(ctx) // Получаем тексты

		if !exists {
			e.svc.SetWaitingName(customerID)
			_, err := botCtx.Bot().SendMessage(ctx, tu.Message(tu.ID(message.Chat.ID), msgs.WelcomeNewUser).WithParseMode(telego.ModeHTML))
			return err
		}

		// Если мы ждали имя — сохраняем его
		if session.WaitingName {
			if message.Text == "" {
				_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(tu.ID(message.Chat.ID), msgs.AskForText).WithParseMode(telego.ModeHTML))
				return nil
			}
			e.svc.SaveName(customerID, message.Text)
			// Обновляем локальную переменную, чтобы сразу выдать меню ниже
			session.FullName = message.Text
		}

		// СЦЕНАРИЙ Г: Имя введено, но топика нет (клиент пишет текст вместо нажатия на кнопку)
		kb, _ := e.svc.GetCategoriesKeyboard(ctx, nil)
		prompt, _ := e.svc.GetMainPrompt(ctx)

		// Безопасно экранируем имя пользователя
		safeName := html.EscapeString(session.FullName)

		var finalPrompt string
		if prompt == "" {
			finalPrompt = fmt.Sprintf("<b>%s</b>, пожалуйста, воспользуйтесь кнопками меню ниже для выбора темы:", safeName)
		} else {
			finalPrompt = fmt.Sprintf("<b>%s</b>, %s", safeName, prompt)
		}

		_, _ = botCtx.Bot().SendMessage(ctx, tu.Message(
			tu.ID(message.Chat.ID),
			finalPrompt,
		).WithReplyMarkup(kb).WithParseMode(telego.ModeHTML))

		return nil
	}, telegram.IsPrivateChat(), e.svc.IsCustomer())

	// 5. ОТВЕТЫ ОТ МЕНЕДЖЕРОВ В ТОПИКАХ (Пересылка клиенту)
	bh.HandleMessage(func(botCtx *th.Context, message telego.Message) error {
		ctx := botCtx.Context()

		// Защита: не пересылаем команды (например, /managers или /clearcache) клиенту,
		// если менеджер случайно написал их внутри топика клиента
		if strings.HasPrefix(message.Text, "/") || strings.HasPrefix(message.Caption, "/") {
			return nil
		}

		// Вызываем бизнес-логику пересылки
		if err := e.svc.HandleManagerMessage(ctx, &message); err != nil {
			log.Printf("Error handling manager message: %v", err)
		}
		return nil
	}, telegram.IsGroupChat())

}
