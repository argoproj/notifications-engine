package cmd

import (
	"bytes"
	"testing"

	"github.com/argoproj/notifications-engine/pkg/util/misc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintFormatterJson(t *testing.T) {
	var out bytes.Buffer
	err := misc.PrintFormatted(map[string]string{
		"foo": "bar",
	}, "json", &out)

	require.NoError(t, err)
	assert.Contains(t, out.String(), `{
  "foo": "bar"
}`)
}

func TestPrintFormatterYaml(t *testing.T) {
	var out bytes.Buffer
	err := misc.PrintFormatted(map[string]string{
		"foo": "bar",
	}, "yaml", &out)

	require.NoError(t, err)
	assert.Contains(t, out.String(), `foo: bar`)
}
