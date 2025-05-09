package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	texttemplate "text/template"
	"time"

	"github.com/google/uuid"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"google.golang.org/api/chat/v1"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
)

type GoogleChatNotification struct {
	Cards     string `json:"cards"`
	CardsV2   string `json:"cardsV2"`
	ThreadKey string `json:"threadKey,omitempty"`
}

type googleChatMessage struct {
	Text    string            `json:"text"`
	Cards   []chat.Card       `json:"cards,omitempty"`
	CardsV2 []chat.CardWithId `json:"cardsV2,omitempty"`
}

func (n *GoogleChatNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	cards, err := texttemplate.New(name).Funcs(f).Parse(n.Cards)
	if err != nil {
		return nil, fmt.Errorf("error in '%s' googlechat.cards : %w", name, err)
	}

	cardsV2, err := texttemplate.New(name).Funcs(f).Parse(n.CardsV2)
	if err != nil {
		return nil, fmt.Errorf("error in '%s' googlechat.cards : %w", name, err)
	}

	threadKey, err := texttemplate.New(name).Funcs(f).Parse(n.ThreadKey)
	if err != nil {
		return nil, fmt.Errorf("error in '%s' googlechat.threadKey : %w", name, err)
	}

	return func(notification *Notification, vars map[string]interface{}) error {
		if notification.GoogleChat == nil {
			notification.GoogleChat = &GoogleChatNotification{}
		}
		var cardsBuff bytes.Buffer
		if err := cards.Execute(&cardsBuff, vars); err != nil {
			return err
		}
		if val := cardsBuff.String(); val != "" {
			notification.GoogleChat.Cards = val
		}

		var cardsV2Buff bytes.Buffer
		if err := cardsV2.Execute(&cardsV2Buff, vars); err != nil {
			return err
		}
		if val := cardsV2Buff.String(); val != "" {
			notification.GoogleChat.CardsV2 = val
		}

		var threadKeyBuff bytes.Buffer
		if err := threadKey.Execute(&threadKeyBuff, vars); err != nil {
			return err
		}
		if val := threadKeyBuff.String(); val != "" {
			notification.GoogleChat.ThreadKey = val
		}

		return nil
	}, nil
}

type GoogleChatOptions struct {
	WebhookUrls         map[string]string `json:"webhooks"`
	InsecureSkipVerify  bool              `json:"insecureSkipVerify"`
	MaxIdleConns        int               `json:"maxIdleConns"`
	MaxIdleConnsPerHost int               `json:"maxIdleConnsPerHost"`
	MaxConnsPerHost     int               `json:"maxConnsPerHost"`
	IdleConnTimeout     time.Duration     `json:"idleConnTimeout"`
}

type googleChatService struct {
	opts GoogleChatOptions
}

func NewGoogleChatService(opts GoogleChatOptions) NotificationService {
	return &googleChatService{opts: opts}
}

type webhookReturn struct {
	Error *webhookError `json:"error"`
}

type webhookError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

func (s googleChatService) getClient(recipient string) (*googlechatClient, error) {
	webhookUrl, ok := s.opts.WebhookUrls[recipient]
	if !ok {
		return nil, fmt.Errorf("no Google chat webhook configured for recipient %s", recipient)
	}
	transport := httputil.NewTransport(webhookUrl, s.opts.MaxIdleConns, s.opts.MaxIdleConnsPerHost, s.opts.MaxConnsPerHost, s.opts.IdleConnTimeout, false)
	client := &http.Client{
		Transport: httputil.NewLoggingRoundTripper(transport, log.WithField("service", "googlechat")),
	}
	return &googlechatClient{httpClient: client, url: webhookUrl}, nil
}

type googlechatClient struct {
	httpClient *http.Client
	url        string
}

func (c *googlechatClient) sendMessage(message *googleChatMessage, threadKey string) (*webhookReturn, error) {
	jsonMessage, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(c.url)
	if err != nil {
		return nil, err
	}
	if threadKey != "" {
		q := u.Query()
		q.Add("threadKey", threadKey)
		q.Add("messageReplyOption", "REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD")
		u.RawQuery = q.Encode()
	}
	response, err := c.httpClient.Post(u.String(), "application/json", bytes.NewReader(jsonMessage))
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = response.Body.Close()
	}()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	body := webhookReturn{}
	err = json.Unmarshal(bodyBytes, &body)
	if err != nil {
		return nil, err
	}
	return &body, nil
}

func (s googleChatService) Send(notification Notification, dest Destination) error {
	client, err := s.getClient(dest.Recipient)
	if err != nil {
		return fmt.Errorf("error creating client to webhook: %w", err)
	}
	message, err := googleChatNotificationToMessage(notification)
	if err != nil {
		return fmt.Errorf("cannot create message: %w", err)
	}

	var threadKey string
	if notification.GoogleChat != nil {
		threadKey = notification.GoogleChat.ThreadKey
	}

	body, err := client.sendMessage(message, threadKey)
	if err != nil {
		return fmt.Errorf("cannot send message: %w", err)
	}
	if body.Error != nil {
		return fmt.Errorf("error with message: code=%d status=%s message=%s", body.Error.Code, body.Error.Status, body.Error.Message)
	}
	return nil
}

func googleChatNotificationToMessage(n Notification) (*googleChatMessage, error) {
	message := &googleChatMessage{}
	if n.GoogleChat != nil && (n.GoogleChat.CardsV2 != "" || n.GoogleChat.Cards != "") {
		if n.GoogleChat.CardsV2 != "" {
			// Unmarshal Modern Type

			var cardData []chat.GoogleAppsCardV1Card
			err := yaml.Unmarshal([]byte(n.GoogleChat.CardsV2), &cardData)
			if err != nil {
				return nil, err
			}

			message.CardsV2 = make([]chat.CardWithId, len(cardData))

			for i, datum := range cardData {
				message.CardsV2[i] = chat.CardWithId{
					CardId: uuid.New().String(),
					Card:   &datum,
				}
			}
		}

		if n.GoogleChat.Cards != "" {
			// Unmarshal Legacy Type
			err := yaml.Unmarshal([]byte(n.GoogleChat.Cards), &message.Cards)
			if err != nil {
				return nil, err
			}
		}
	} else {
		message.Text = n.Message
	}
	return message, nil
}
