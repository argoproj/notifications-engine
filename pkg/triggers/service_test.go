package triggers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	svc, err := NewService(map[string][]Condition{
		"my-trigger": {{
			When: "var1 == 'abc'",
			Send: []string{"my-template"},
		}},
	})

	require.NoError(t, err)

	conditionKey := "[0]." + hash("var1 == 'abc'")

	t.Run("Triggered", func(t *testing.T) {
		res, err := svc.Run("my-trigger", map[string]any{"var1": "abc"})
		if assert.NoError(t, err) {
			return
		}
		assert.Equal(t, []ConditionResult{{
			Key:       conditionKey,
			Triggered: true,
			Templates: []string{"my-template"},
		}}, res)
	})

	t.Run("NotTriggered", func(t *testing.T) {
		res, err := svc.Run("my-trigger", map[string]any{"var1": "bcd"})
		if assert.NoError(t, err) {
			return
		}
		assert.Equal(t, []ConditionResult{{
			Key:       conditionKey,
			Triggered: false,
			Templates: []string{"my-template"},
		}}, res)
	})

	t.Run("Failed", func(t *testing.T) {
		res, err := svc.Run("my-trigger", map[string]any{})
		if assert.NoError(t, err) {
			return
		}
		assert.Equal(t, []ConditionResult{{
			Key:       conditionKey,
			Triggered: false,
			Templates: []string{"my-template"},
		}}, res)
	})
}

func TestRun_OncePerSet(t *testing.T) {
	revision := "123"
	svc, err := NewService(map[string][]Condition{
		"my-trigger": {{
			When:    "var1 == 'abc'",
			Send:    []string{"my-template"},
			OncePer: "revision",
		}},
	})

	require.NoError(t, err)

	conditionKey := "[0]." + hash("var1 == 'abc'")

	t.Run("Triggered", func(t *testing.T) {
		res, err := svc.Run("my-trigger", map[string]any{"var1": "abc", "revision": "123"})
		require.NoError(t, err)
		assert.Equal(t, []ConditionResult{{
			Key:       conditionKey,
			Triggered: true,
			Templates: []string{"my-template"},
			OncePer:   revision,
		}}, res)
	})

	t.Run("NotTriggered", func(t *testing.T) {
		res, err := svc.Run("my-trigger", map[string]any{"var1": "bcd"})
		require.NoError(t, err)
		assert.Equal(t, []ConditionResult{{
			Key:       conditionKey,
			Triggered: false,
			Templates: []string{"my-template"},
			OncePer:   "",
		}}, res)
	})
}

func TestRun_OncePer_Evaluate(t *testing.T) {
	vars := map[string]any{
		"var1":     "abc",
		"revision": "123",
		"app": map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					"example.com/version": "v0.1",
				},
			},
		},
	}

	tests := []struct {
		OncePer string
		Result  string
	}{
		{
			OncePer: "revision",
			Result:  "123",
		},
		{
			OncePer: `app.metadata.annotations["example.com/version"]`,
			Result:  "v0.1",
		},
	}

	for _, tt := range tests {
		svc, err := NewService(map[string][]Condition{
			"my-trigger": {{
				When:    "var1 == 'abc'",
				Send:    []string{"my-template"},
				OncePer: tt.OncePer,
			}},
		})

		require.NoError(t, err)

		conditionKey := "[0]." + hash("var1 == 'abc'")

		res, err := svc.Run("my-trigger", vars)
		require.NoError(t, err)

		assert.Equal(t, []ConditionResult{{
			Key:       conditionKey,
			Triggered: true,
			Templates: []string{"my-template"},
			OncePer:   tt.Result,
		}}, res)
	}
}
