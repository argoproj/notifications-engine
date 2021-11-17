package services

import (
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type TelegramOptions struct {
	Token string `json:"token"`
}

func NewTelegramService(opts TelegramOptions) NotificationService {
	return &telegramService{opts: opts}
}

type telegramService struct {
	opts TelegramOptions
}

func (s telegramService) Send(notification Notification, dest Destination) error {
	bot, err := tgbotapi.NewBotAPI(s.opts.Token)
	if err != nil {
		return err
	}

	if strings.HasPrefix(dest.Recipient, "-") {
		chatID, err := strconv.ParseInt(dest.Recipient, 10, 64)
		if err != nil {
			return err
		}

		// Init message with ParseMode is 'Markdown'
		msg := tgbotapi.NewMessage(chatID, notification.Message)
		msg.ParseMode = "Markdown"

		_, err = bot.Send(msg)
		if err != nil {
			return err
		}
	} else {
		// Init message with ParseMode is 'Markdown'
		msg := tgbotapi.NewMessageToChannel("@"+dest.Recipient, notification.Message)
		msg.ParseMode = "Markdown"

		_, err := bot.Send(msg)
		if err != nil {
			return err
		}
	}

	return nil
}
