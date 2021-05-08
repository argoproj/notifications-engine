package controller

import (
	"strconv"
	"testing"

	"github.com/argoproj/notifications-engine/pkg/triggers"

	"github.com/argoproj/notifications-engine/pkg/services"

	"github.com/stretchr/testify/assert"
)

func TestNotificationState_Truncate(t *testing.T) {
	state := emptyNotificationsState()
	for i := 0; i < 5; i++ {
		state.NotifiedState[strconv.Itoa(i)] = int64(i)
	}

	state.truncate(3)

	assert.Equal(t, NotificationsState{
		NotifiedState: map[string]int64{
			"2": 2, "3": 3, "4": 4,
		},
		ServiceState: map[string]int64{},
	}, state)
}

func TestSetAlreadyNotified(t *testing.T) {
	dest := services.Destination{Service: "slack", Recipient: "my-channel"}

	state := emptyNotificationsState()
	changed := state.SetAlreadyNotified("app-synced", triggers.ConditionResult{Key: "0"}, dest, true)

	assert.True(t, changed)
	_, ok := state.NotifiedState["app-synced:0:slack:my-channel"]
	assert.True(t, ok)

	changed = state.SetAlreadyNotified("app-synced", triggers.ConditionResult{Key: "0"}, dest, true)
	assert.False(t, changed)

	changed = state.SetAlreadyNotified("app-synced", triggers.ConditionResult{Key: "0"}, dest, false)
	assert.True(t, changed)
	_, ok = state.NotifiedState["app-synced:0:slack:my-channel"]
	assert.False(t, ok)
}

func TestSetAlreadyNotified_OncePerItem(t *testing.T) {
	dest := services.Destination{Service: "slack", Recipient: "my-channel"}

	state := emptyNotificationsState()
	changed := state.SetAlreadyNotified("app-synced", triggers.ConditionResult{OncePer: "abc", Key: "0"}, dest, true)

	assert.True(t, changed)
	_, ok := state.NotifiedState["abc:app-synced:0:slack:my-channel"]
	assert.True(t, ok)

	changed = state.SetAlreadyNotified("app-synced", triggers.ConditionResult{OncePer: "abc", Key: "0"}, dest, false)
	assert.False(t, changed)
	_, ok = state.NotifiedState["abc:app-synced:0:slack:my-channel"]
	assert.True(t, ok)
}
