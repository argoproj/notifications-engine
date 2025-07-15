package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateNotifyConsole(t *testing.T) {
	cmData := map[string]string{
		"trigger.my-trigger": `[{when: "app.metadata.name == 'guestbook'", send: [my-template]}]`,
		"template.my-template": `
message: hello {{.app.metadata.name}}`,
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	ctx, closer, err := newTestContext(&stdout, &stderr, cmData, newTestResource("guestbook"))
	require.NoError(t, err)
	defer closer()

	command := newTemplateNotifyCommand(ctx)
	err = command.RunE(command, []string{"my-template", "guestbook"})
	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Contains(t, stdout.String(), "hello guestbook")
}

func TestTemplateGet(t *testing.T) {
	cmData := map[string]string{
		"template.my-template1": `{message: hello}`,
		"template.my-template2": `{message: hello}`,
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	ctx, closer, err := newTestContext(&stdout, &stderr, cmData)
	require.NoError(t, err)
	defer closer()

	command := newTemplateGetCommand(ctx)
	err = command.RunE(command, nil)
	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Contains(t, stdout.String(), "my-template1")
	assert.Contains(t, stdout.String(), "my-template2")
}
