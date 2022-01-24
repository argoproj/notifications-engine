package slack

import (
	"context"
	"encoding/json"
	"reflect"
	"runtime"
	"strconv"
	"strings"
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

type msgOptionMatcher struct {
	name string
}

func (m msgOptionMatcher) Matches(option interface{}) bool {
	name := runtime.FuncForPC(reflect.ValueOf(option).Pointer()).Name()
	return strings.Contains(name, m.name)
}

func (m msgOptionMatcher) String() string {
	return m.name
}

func EqMsgOption(name string) gomock.Matcher {
	return msgOptionMatcher{name}
}

func TestThreadedClient(t *testing.T) {
	const (
		groupingKey string = "group"
		channel     string = "channel"
		ts1         string = "1"
		ts2         string = "2"
	)

	tests := map[string]struct {
		threadTSs     timestampMap
		groupingKey   string
		policy        DeliveryPolicy
		wantOpt1      string
		wantOpt2      string
		wantthreadTSs timestampMap
	}{
		"Post, basic": {
			threadTSs:     timestampMap{},
			groupingKey:   "",
			policy:        Post,
			wantOpt1:      "MsgOptionPost",
			wantOpt2:      "",
			wantthreadTSs: timestampMap{},
		},
		"Post, no parent, with grouping": {
			threadTSs:     timestampMap{},
			groupingKey:   groupingKey,
			policy:        Post,
			wantOpt1:      "MsgOptionPost",
			wantOpt2:      "",
			wantthreadTSs: timestampMap{channel: {groupingKey: ts1}},
		},
		"Post, with parent, with grouping": {
			threadTSs:     timestampMap{channel: {groupingKey: ts2}},
			groupingKey:   groupingKey,
			policy:        Post,
			wantOpt1:      "MsgOptionPost",
			wantOpt2:      "",
			wantthreadTSs: timestampMap{channel: {groupingKey: ts2}},
		},
		"PostAndUpdate, no parent. First post should not be updated": {
			threadTSs:     timestampMap{},
			groupingKey:   groupingKey,
			policy:        PostAndUpdate,
			wantOpt1:      "MsgOptionPost",
			wantOpt2:      "",
			wantthreadTSs: timestampMap{channel: {groupingKey: ts1}},
		},
		"PostAndUpdate, with parent. First post should be updated": {
			threadTSs:     timestampMap{channel: {groupingKey: ts2}},
			groupingKey:   groupingKey,
			policy:        PostAndUpdate,
			wantOpt1:      "MsgOptionPost",
			wantOpt2:      "MsgOptionUpdate",
			wantthreadTSs: timestampMap{channel: {groupingKey: ts2}},
		},
		"Update, no parent. Only call should be post": {
			threadTSs:     timestampMap{},
			groupingKey:   groupingKey,
			policy:        Update,
			wantOpt1:      "MsgOptionPost",
			wantOpt2:      "",
			wantthreadTSs: timestampMap{channel: {groupingKey: ts1}},
		},
		"Update, with parent. Only call should be update": {
			threadTSs:     timestampMap{channel: {groupingKey: ts2}},
			groupingKey:   groupingKey,
			policy:        Update,
			wantOpt1:      "MsgOptionUpdate",
			wantOpt2:      "",
			wantthreadTSs: timestampMap{channel: {groupingKey: ts2}},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := mocks.NewMockSlackClient(ctrl)

			m.EXPECT().
				SendMessageContext(gomock.Any(), gomock.Any(), EqMsgOption(tc.wantOpt1)).
				Return(channel, ts1, "", nil)

			if tc.wantOpt2 != "" {
				m.EXPECT().
					SendMessageContext(gomock.Any(), gomock.Any(), EqMsgOption(tc.wantOpt2))
			}

			client := NewThreadedClient(m, &state{rate.NewLimiter(rate.Inf, 1), tc.threadTSs, channelMap{}})
			err := client.SendMessage(context.TODO(), channel, tc.groupingKey, false, tc.policy, []slack.MsgOption{})
			assert.NoError(t, err)
			assert.Equal(t, tc.wantthreadTSs, client.ThreadTSs)
		})
	}
}
