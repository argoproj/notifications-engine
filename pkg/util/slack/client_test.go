package slack

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/argoproj/notifications-engine/pkg/util/slack/mocks"

	"github.com/golang/mock/gomock"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func TestDeliveryPolicy_MarshalJSON(t *testing.T) {
	tests := []struct {
		input DeliveryPolicy
		want  string
	}{
		{input: Post, want: `"Post"`},
		{input: PostAndUpdate, want: `"PostAndUpdate"`},
		{input: Update, want: `"Update"`},
		{input: 100, want: `"Post"`},
	}

	for i, tc := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			bs, err := json.Marshal(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, string(bs))
		})
	}
}

func TestDeliveryPolicy_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		input string
		want  DeliveryPolicy
	}{
		{input: `"Post"`, want: Post},
		{input: `"PostAndUpdate"`, want: PostAndUpdate},
		{input: `"Update"`, want: Update},
		{input: `"Error"`, want: Post},
	}

	for i, tc := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var got DeliveryPolicy
			err := json.Unmarshal([]byte(tc.input), &got)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// Checks what api method a slack API call will use.
//
// https://api.slack.com/methods/chat.postMessage
// https://api.slack.com/methods/chat.update
type slackAPIMethodMatcher struct {
	wantAPIMethod string
}

func (m slackAPIMethodMatcher) Matches(maybeMsgOption interface{}) bool {
	msgOption, ok := maybeMsgOption.(slack.MsgOption)
	if !ok {
		return false
	}
	// Use utility function for debugging/testing chat requests. Specify an
	// empty apiurl so we only get the endpoint method.
	endpoint, _, err := slack.UnsafeApplyMsgOptions("token", "channel", "", msgOption)
	if err != nil {
		return false
	}
	return m.wantAPIMethod == endpoint
}

func (m slackAPIMethodMatcher) String() string {
	return "MsgOption for " + m.wantAPIMethod
}

func EqChatUpdate() gomock.Matcher {
	return slackAPIMethodMatcher{"chat.update"}
}

func EqChatPost() gomock.Matcher {
	return slackAPIMethodMatcher{"chat.postMessage"}
}

func TestThreadedClient(t *testing.T) {
	const (
		groupingKey string = "group"
		channel     string = "channel"
		channelID   string = "channel-ID"
		ts1         string = "1"
		ts2         string = "2"
	)

	tests := map[string]struct {
		threadTSs     timestampMap
		groupingKey   string
		policy        DeliveryPolicy
		wantPostType1 gomock.Matcher
		wantThreadTSs timestampMap
	}{
		"Post, basic": {
			threadTSs:     timestampMap{},
			groupingKey:   "",
			policy:        Post,
			wantPostType1: EqChatPost(),
			wantThreadTSs: timestampMap{},
		},
		"Post, no parent, with grouping": {
			threadTSs:     timestampMap{},
			groupingKey:   groupingKey,
			policy:        Post,
			wantPostType1: EqChatPost(),
			wantThreadTSs: timestampMap{channel: {groupingKey: ts1}},
		},
		"Post, with parent, with grouping": {
			threadTSs:     timestampMap{channel: {groupingKey: ts2}},
			groupingKey:   groupingKey,
			policy:        Post,
			wantPostType1: EqChatPost(),
			wantThreadTSs: timestampMap{channel: {groupingKey: ts2}},
		},
		"PostAndUpdate, no parent. First post should not be updated": {
			threadTSs:     timestampMap{},
			groupingKey:   groupingKey,
			policy:        PostAndUpdate,
			wantPostType1: EqChatPost(),
			wantThreadTSs: timestampMap{channel: {groupingKey: ts1}},
		},
		"PostAndUpdate, with parent. First post should be updated": {
			threadTSs:     timestampMap{channel: {groupingKey: ts2}},
			groupingKey:   groupingKey,
			policy:        PostAndUpdate,
			wantPostType1: EqChatUpdate(),
			wantThreadTSs: timestampMap{channel: {groupingKey: ts2}},
		},
		"Update, no parent. There should be no call, no new thread": {
			threadTSs:     timestampMap{},
			groupingKey:   groupingKey,
			policy:        Update,
			wantPostType1: nil,
			wantThreadTSs: timestampMap{},
		},
		"Update, with parent. Only call should be update": {
			threadTSs:     timestampMap{channel: {groupingKey: ts2}},
			groupingKey:   groupingKey,
			policy:        Update,
			wantPostType1: EqChatUpdate(),
			wantThreadTSs: timestampMap{channel: {groupingKey: ts2}},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := mocks.NewMockSlackClient(ctrl)

			expectedFunctionCall := m.EXPECT().
				SendMessageContext(gomock.Any(), gomock.Eq(channel), tc.wantPostType1)

			if tc.wantPostType1 != nil {
				expectedFunctionCall.Return(channelID, ts1, "", nil)
			} else {
				expectedFunctionCall.MaxTimes(0)
			}

			client := NewThreadedClient(m, &state{rate.NewLimiter(rate.Inf, 1), tc.threadTSs, channelMap{}})
			err := client.SendMessage(context.TODO(), channel, tc.groupingKey, false, tc.policy, []slack.MsgOption{})
			assert.NoError(t, err)
			assert.Equal(t, tc.wantThreadTSs, client.ThreadTSs)
		})
	}
}
