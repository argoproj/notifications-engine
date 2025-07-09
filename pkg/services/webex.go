package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
)

type WebexOptions struct {
	Token               string `json:"token"`
	ApiURL              string `json:"apiURL"`
	InsecureSkipVerify  bool   `json:"insecureSkipVerify"`
	MaxIdleConns        int    `json:"maxIdleConns"`
	MaxIdleConnsPerHost int    `json:"maxIdleConnsPerHost"`
	MaxConnsPerHost     int    `json:"maxConnsPerHost"`
	IdleConnTimeout     string `json:"idleConnTimeout"`
}

type webexService struct {
	opts WebexOptions
}

type webexMessage struct {
	ToPersonEmail string `json:"toPersonEmail,omitempty"`
	RoomId        string `json:"roomId,omitempty"`
	Markdown      string `json:"markdown,omitempty"`
}

func NewWebexService(opts WebexOptions) NotificationService {
	if opts.ApiURL == "" {
		opts.ApiURL = "https://webexapis.com"
	} else {
		opts.ApiURL = strings.TrimSuffix(opts.ApiURL, "/")
	}
	return &webexService{opts: opts}
}

var validEmail = regexp.MustCompile(`^\S+@\S+\.\S+$`)

func (w webexService) Send(notification Notification, dest Destination) error {
	requestURL := fmt.Sprintf("%s/v1/messages", w.opts.ApiURL)

	idleConnTimeout, err := time.ParseDuration(w.opts.IdleConnTimeout)
	if err != nil {
		return fmt.Errorf("failed to parse idle connection timeout")
	}
	client := &http.Client{
		Transport: httputil.NewLoggingRoundTripper(
			httputil.NewTransport(requestURL, w.opts.MaxIdleConns, w.opts.MaxIdleConnsPerHost, w.opts.MaxConnsPerHost, idleConnTimeout, false), log.WithField("service", dest.Service)),
	}

	message := webexMessage{
		Markdown: notification.Message,
	}

	// Set recipient to person or room
	if validEmail.MatchString(dest.Recipient) {
		message.ToPersonEmail = dest.Recipient
	} else {
		message.RoomId = dest.Recipient
	}

	jsonValue, err := json.Marshal(message)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewBuffer(jsonValue))
	if err != nil {
		return err
	}

	apiToken := fmt.Sprintf("Bearer %s", w.opts.Token)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", apiToken)

	response, err := client.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		_ = response.Body.Close()
	}()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("unable to read response data: %v", err)
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("request to %s has failed with error code %d : %s", requestURL, response.StatusCode, string(data))
	}

	return nil
}
