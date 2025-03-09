package services

import (
	"reflect"
	"testing"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
)

func TestBuildTelegramMessageOptions(t *testing.T) {
	tests := []struct {
		name         string
		notification Notification
		dest         Destination
		want         *tgbotapi.MessageConfig
		wantErr      bool
	}{
		{
			name:         "Message to chat",
			notification: Notification{Message: "Test message"},
			dest:         Destination{Recipient: "-123456"},
			want:         &tgbotapi.MessageConfig{Text: "Test message", BaseChat: tgbotapi.BaseChat{ChatConfig: tgbotapi.ChatConfig{ChatID: -123456}, MessageThreadID: 0}, ParseMode: "Markdown"},
			wantErr:      false,
		},
		{
			name:         "Message to chat with thread",
			notification: Notification{Message: "Test message"},
			dest:         Destination{Recipient: "-123456|1"},
			want:         &tgbotapi.MessageConfig{Text: "Test message", BaseChat: tgbotapi.BaseChat{ChatConfig: tgbotapi.ChatConfig{ChatID: -123456}, MessageThreadID: 1}, ParseMode: "Markdown"},
			wantErr:      false,
		},
		{
			name:         "Message to channel",
			notification: Notification{Message: "Test message"},
			dest:         Destination{Recipient: "123456"},
			want:         &tgbotapi.MessageConfig{Text: "Test message", BaseChat: tgbotapi.BaseChat{ChatConfig: tgbotapi.ChatConfig{ChannelUsername: "@123456"}}, ParseMode: "Markdown"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildTelegramMessageOptions(tt.notification, tt.dest)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildTelegramMessageOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildTelegramMessageOptions() = %v, want %v", got, tt.want)
			}
		})
	}
}
