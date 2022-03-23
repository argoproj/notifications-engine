package controller

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/argoproj/notifications-engine/pkg/subscriptions"
	"github.com/argoproj/notifications-engine/pkg/triggers"
)

const (
	notifiedHistoryMaxSize = 100
	NotifiedAnnotationKey  = "notified." + subscriptions.AnnotationPrefix
	ServiceAnnotationKey   = "notified." + subscriptions.AnnotationPrefix + "/service"
)

func StateItemKey(trigger string, conditionResult triggers.ConditionResult, dest services.Destination) string {
	key := fmt.Sprintf("%s:%s:%s:%s", trigger, conditionResult.Key, dest.Service, dest.Recipient)
	if conditionResult.OncePer != "" {
		key = conditionResult.OncePer + ":" + key
	}
	return key
}

type NotificationsState struct {
	// NotifiedState track notification triggers state (already notified/not notified)
	NotifiedState map[string]int64
	ServiceState  map[string]int64
}

// truncate ensures that state has no more than specified number of items and
// removes unnecessary items starting from oldest
func (s NotificationsState) truncate(maxSize int) {
	if cnt := len(s.NotifiedState) - maxSize; cnt > 0 {
		var keys []string
		for k := range s.NotifiedState {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return s.NotifiedState[keys[i]] < s.NotifiedState[keys[j]]
		})

		for i := 0; i < cnt; i++ {
			delete(s.NotifiedState, keys[i])
		}
	}
}

// SetAlreadyNotified set the state of given trigger/destination and return if state has been changed
func (s NotificationsState) SetAlreadyNotified(trigger string, result triggers.ConditionResult, dest services.Destination, isNotified bool) bool {
	key := StateItemKey(trigger, result, dest)
	if _, alreadyNotified := s.NotifiedState[key]; alreadyNotified == isNotified {
		return false
	}
	if isNotified {
		s.NotifiedState[key] = time.Now().Unix()
	} else {
		if result.OncePer != "" {
			return false
		}
		delete(s.NotifiedState, key)
	}
	return true
}

// SkipFirstRun set the state of given destination and return if state hasn't set
func (s NotificationsState) SkipFirstRun(dest services.Destination) bool {
	key := fmt.Sprintf("%s:%s", dest.Service, dest.Recipient)
	if _, ok := s.ServiceState[key]; ok {
		return false
	}

	s.ServiceState[key] = time.Now().Unix()
	return true
}

func (s NotificationsState) Persist(res metav1.Object) (map[string]string, error) {
	s.truncate(notifiedHistoryMaxSize)

	annotations := map[string]string{}

	if res.GetAnnotations() != nil {
		for k, v := range res.GetAnnotations() {
			annotations[k] = v
		}
	}

	if len(s.NotifiedState) == 0 {
		delete(annotations, NotifiedAnnotationKey)
	} else {
		stateJson, err := json.Marshal(s.NotifiedState)
		if err != nil {
			return nil, err
		}
		annotations[NotifiedAnnotationKey] = string(stateJson)
	}

	if len(s.ServiceState) == 0 {
		delete(annotations, ServiceAnnotationKey)
	} else {
		stateJson, err := json.Marshal(s.ServiceState)
		if err != nil {
			return nil, err
		}
		annotations[ServiceAnnotationKey] = string(stateJson)
	}

	return annotations, nil
}

func newState(notifiedVal, serviceVal string) NotificationsState {
	res := emptyNotificationsState()

	if notifiedVal != "" {
		if err := json.Unmarshal([]byte(notifiedVal), &res.NotifiedState); err != nil {
			return emptyNotificationsState()
		}
	}

	if serviceVal != "" {
		if err := json.Unmarshal([]byte(serviceVal), &res.ServiceState); err != nil {
			return emptyNotificationsState()
		}
	}

	return res
}

func NewStateFromRes(res metav1.Object) NotificationsState {
	if annotations := res.GetAnnotations(); annotations != nil {
		return newState(annotations[NotifiedAnnotationKey], annotations[ServiceAnnotationKey])
	}
	return emptyNotificationsState()
}

func emptyNotificationsState() NotificationsState {
	return NotificationsState{
		NotifiedState: map[string]int64{},
		ServiceState:  map[string]int64{},
	}
}
