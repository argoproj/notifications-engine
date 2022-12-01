package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidEmail(t *testing.T) {
	assert.Equal(t, true, validEmail.MatchString("test@test.com"))
	assert.Equal(t, true, validEmail.MatchString("test.test@test.com"))
	assert.Equal(t, false, validEmail.MatchString("notAnEmail"))
	assert.Equal(t, false, validEmail.MatchString("notAnEmail@"))
}
