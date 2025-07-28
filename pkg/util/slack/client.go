package slack

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	sl "github.com/slack-go/slack"
	"golang.org/x/time/rate"
)

type DeliveryPolicy int

const (
	Post DeliveryPolicy = iota
	PostAndUpdate
	Update
)

func (p DeliveryPolicy) String() string {
	switch p {
	case Post:
		return "Post"
	case PostAndUpdate:
		return "PostAndUpdate"
	case Update:
		return "Update"
	}
	return "Post"
}

func (p DeliveryPolicy) FromString(policy string) DeliveryPolicy {
	switch policy {
	case "Post":
		return Post
	case "PostAndUpdate":
		return PostAndUpdate
	case "Update":
		return Update
	}
	return Post
}

func (p DeliveryPolicy) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

func (p *DeliveryPolicy) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*p = p.FromString(s)
	return nil
}

//go:generate mockgen -destination=./mocks/client.go -source=$GOFILE -package=mocks SlackClient
type SlackClient interface {
	SendMessageContext(ctx context.Context, channelID string, options ...sl.MsgOption) (string, string, string, error)
}

type timestampMap map[string]map[string]string
type channelMap map[string]string

type state struct {
	lock sync.Mutex

	Limiter    *rate.Limiter
	ThreadTSs  timestampMap
	ChannelIDs channelMap
}

func NewState(limiter *rate.Limiter) *state {
	return &state{
		Limiter:    limiter,
		ThreadTSs:  make(timestampMap),
		ChannelIDs: make(channelMap),
	}
}

type threadedClient struct {
	Client SlackClient
	*state
}

func NewThreadedClient(client SlackClient, s *state) *threadedClient {
	return &threadedClient{client, s}
}

func (c *threadedClient) getChannelID(recipient string) string {
	c.lock.Lock()
	defer c.lock.Unlock()

	if id, ok := c.ChannelIDs[recipient]; ok {
		return id
	}
	return recipient
}

func (c *threadedClient) getThreadTimestamp(recipient string, groupingKey string) string {
	c.lock.Lock()
	defer c.lock.Unlock()

	thread, ok := c.ThreadTSs[recipient]
	if !ok {
		return ""
	}
	return thread[groupingKey]
}

func (c *threadedClient) setThreadTimestamp(recipient string, groupingKey string, ts string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	thread, ok := c.ThreadTSs[recipient]
	if !ok {
		thread = map[string]string{}
		c.ThreadTSs[recipient] = thread
	}
	thread[groupingKey] = ts
}

func (c *threadedClient) SendMessage(ctx context.Context, recipient string, groupingKey string, broadcast bool, policy DeliveryPolicy, options []sl.MsgOption) error {
	ts := c.getThreadTimestamp(recipient, groupingKey)
	if groupingKey != "" && ts != "" {
		options = append(options, sl.MsgOptionTS(ts))
	}

	if ts == "" || policy == Post || policy == PostAndUpdate {
		newTs, channelID, err := SendMessageRateLimited(
			c.Client,
			ctx,
			c.Limiter,
			recipient,
			sl.MsgOptionPost(),
			buildPostOptions(broadcast, options),
		)
		if err != nil {
			return err
		}
		if groupingKey != "" && ts == "" {
			c.setThreadTimestamp(recipient, groupingKey, newTs)
		}

		c.lock.Lock()
		c.ChannelIDs[recipient] = channelID
		c.lock.Unlock()
	}

	if ts != "" && (policy == Update || policy == PostAndUpdate) {
		_, _, err := SendMessageRateLimited(
			c.Client,
			ctx,
			c.Limiter,
			c.getChannelID(recipient),
			sl.MsgOptionUpdate(ts),
			sl.MsgOptionAsUser(true),
			sl.MsgOptionCompose(options...),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func buildPostOptions(broadcast bool, options []sl.MsgOption) sl.MsgOption {
	opt := sl.MsgOptionCompose(options...)
	if broadcast {
		opt = sl.MsgOptionCompose(sl.MsgOptionBroadcast(), opt)
	}
	return opt
}

func SendMessageRateLimited(client SlackClient, ctx context.Context, limiter *rate.Limiter, recipient string, options ...sl.MsgOption) (ts, channelID string, err error) {
	for {
		err = limiter.Wait(ctx)
		if err != nil {
			break
		}

		channelID, ts, _, err = client.SendMessageContext(
			ctx,
			recipient,
			options...,
		)

		if err != nil {
			var rateLimitedError *sl.RateLimitedError
			if errors.As(err, &rateLimitedError) {
				limiter.SetLimit(rate.Every(rateLimitedError.RetryAfter))
			} else {
				break
			}
		} else {
			// No error, so remove rate limit
			limiter.SetLimit(rate.Inf)
			break
		}
	}
	return
}
