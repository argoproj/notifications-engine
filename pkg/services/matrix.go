package services

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

type MatrixOptions struct {
	AccessToken   string      `json:"accessToken"`
	DeviceID      id.DeviceID `json:"deviceID"`
	HomeserverURL string      `json:"homeserverURL,omitempty"`
	UserID        id.UserID   `json:"userID"`
}

func NewMatrixService(opts MatrixOptions) (NotificationService, error) {
	homeserverURL := opts.HomeserverURL
	if homeserverURL == "" {
		_, serverName, err := opts.UserID.Parse()
		if err != nil {
			return nil, fmt.Errorf("couldn't parse user ID '%s' for server name: %w", opts.UserID, err)
		}
		resp, err := mautrix.DiscoverClientAPI(serverName)
		if err != nil {
			return nil, fmt.Errorf("couldn't discover client URL for homeserver '%s'; try setting matrix.homeserverURL: %w", serverName, err)
		}
		homeserverURL = resp.Homeserver.BaseURL
	}
	client, err := mautrix.NewClient(homeserverURL, opts.UserID, opts.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create matrix client: %w", err)
	}
	// normally gets set during client.Login
	client.DeviceID = opts.DeviceID
	return &matrixService{client, opts}, nil
}

type matrixService struct {
	client *mautrix.Client
	opts   MatrixOptions
}

func (s *matrixService) Send(notification Notification, dest Destination) error {
	if len(dest.Recipient) == 0 {
		return fmt.Errorf("destination cannot be empty")
	}

	// assume destination is a room ID
	roomID := id.RoomID(dest.Recipient)
	serverName := ""

	// check if destination is instead a room alias
	if dest.Recipient[0] == '#' {
		// resolve room alias to room ID
		roomAlias := id.RoomAlias(dest.Recipient)
		resp, err := s.client.ResolveAlias(roomAlias)
		if err != nil {
			return fmt.Errorf("couldn't resolve room alias '%s': %w", dest.Recipient, err)
		}
		roomID = resp.RoomID
		roomAliasStr := roomAlias.String()
		if i := strings.Index(roomAliasStr, ":"); i >= 0 {
			serverName = roomAliasStr[i+1:]
		} else {
			serverName = ""
		}
	}

	markdownContent := format.RenderMarkdown(notification.Message, true, true)

	resp, err := s.client.JoinedRooms()
	if err != nil {
		log.Errorf("couldn't fetch list of joined rooms; will attempt to send message regardless: %s", err)
	} else {
		hasJoined := false
		for _, joinedRoomID := range resp.JoinedRooms {
			if joinedRoomID == roomID {
				hasJoined = true
				break
			}
		}
		if !hasJoined {
			_, err := s.client.JoinRoom(roomID.String(), serverName, nil)
			if err != nil {
				return fmt.Errorf("couldn't join room '%s': %w", roomID, err)
			}
		}
	}

	_, err = s.client.SendMessageEvent(roomID, event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgNotice,
		Body:    markdownContent.Body,

		Format:        markdownContent.Format,
		FormattedBody: markdownContent.FormattedBody,
	})
	if err != nil {
		return fmt.Errorf("couldn't send matrix message: %w", err)
	}
	return nil
}
