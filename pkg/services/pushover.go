package services

import (
	"github.com/gregdel/pushover"
)

type PushoverOptions struct {
	Token string `json:"token"`
}

type pushoverService struct {
	opts PushoverOptions
}

func NewPushoverService(opts PushoverOptions) NotificationService {
	return &pushoverService{opts: opts}
}

func (s *pushoverService) Send(notification Notification, dest Destination) error {
	app := pushover.New(s.opts.Token)

	recipient := pushover.NewRecipient(dest.Recipient)

	message := pushover.NewMessage(notification.Message)

	_, err := app.SendMessage(message, recipient)

	return err
}
