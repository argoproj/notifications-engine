package services

import (
	"strconv"
	"strings"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
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

func buildTelegramMessageOptions(notification Notification, dest Destination) (*tgbotapi.MessageConfig, error) {
	msg := tgbotapi.MessageConfig{}

	if strings.HasPrefix(dest.Recipient, "-") {
		chatChannel := strings.Split(dest.Recipient, "|")

		chatID, err := strconv.ParseInt(chatChannel[0], 10, 64)
		if err != nil {
			return nil, err
		}

		// Init message with ParseMode is 'Markdown'
		msg = tgbotapi.NewMessage(chatID, notification.Message)
		msg.ParseMode = "Markdown"

		if len(chatChannel) > 1 {
			threadID, err := strconv.Atoi(chatChannel[1])
			if err != nil {
				return nil, err
			}
			msg.MessageThreadID = threadID
		}
	} else {
		// Init message with ParseMode is 'Markdown'
		msg = tgbotapi.NewMessageToChannel("@"+dest.Recipient, notification.Message)
		msg.ParseMode = "Markdown"
	}
	return &msg, nil
}

func (s telegramService) Send(notification Notification, dest Destination) error {
	bot, err := tgbotapi.NewBotAPI(s.opts.Token)
	if err != nil {
		return err
	}

	msg, err := buildTelegramMessageOptions(notification, dest)
	if err != nil {
		return err
	}

	_, err = bot.Send(msg)
	if err != nil {
		return err
	}

	return nil
}
