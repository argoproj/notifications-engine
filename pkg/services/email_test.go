package services

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gomodules.xyz/notify"
	"k8s.io/utils/strings/slices"
)

func TestGetTemplater_Email(t *testing.T) {
	n := Notification{
		Email: &EmailNotification{
			Subject: "{{.foo}}", Body: "{{.bar}}",
		},
	}

	templater, err := n.GetTemplater("", template.FuncMap{})
	require.NoError(t, err)

	var notification Notification

	err = templater(&notification, map[string]any{
		"foo": "hello",
		"bar": "world",
	})

	require.NoError(t, err)

	assert.Equal(t, "hello", notification.Email.Subject)
	assert.Equal(t, "world", notification.Email.Body)
}

type mockClient struct {
	notify.ByEmail
}

func (c *mockClient) Send() error {
	return nil
}

func (c *mockClient) SendHtml() error {
	return nil
}

func (c *mockClient) WithSubject(string) notify.ByEmail {
	return c
}

func (c *mockClient) WithBody(string) notify.ByEmail {
	return c
}

func (c *mockClient) To(string, ...string) notify.ByEmail {
	return c
}

func TestSend_SingleRecepient(t *testing.T) {
	es := emailService{&mockClient{}, false}
	err := es.Send(Notification{}, Destination{Recipient: "test@email.com"})
	if err != nil {
		t.Error("Error while sending email")
	}
}

func TestSend_MultipleRecepient(t *testing.T) {
	es := emailService{&mockClient{}, true}
	// two email addresses
	err := es.Send(Notification{}, Destination{Recipient: "test1@email.com,test2@email.com"})
	if err != nil {
		t.Error("Error while sending email")
	}
	// three email addresses
	err = es.Send(Notification{}, Destination{Recipient: "test1@email.com,test2@email.com,test3@email.com"})
	if err != nil {
		t.Error("Error while sending email")
	}
}

func TestNewEmailService(t *testing.T) {
	es := NewEmailService(EmailOptions{Html: true})
	if !es.html {
		t.Error("Html set incorrectly")
	}
}

func TestParseTo(t *testing.T) {
	es := emailService{}
	testCases := []struct {
		recipient string
		want      []string
	}{
		{"email1@email.com", []string{"email1@email.com"}},
		{" email1@email.com ", []string{"email1@email.com"}},
		{" email1@email.com  , email2@email.com,email3@email.com", []string{"email1@email.com", "email2@email.com", "email3@email.com"}},
	}
	for _, testCase := range testCases {
		got := es.parseTo(testCase.recipient)
		if !slices.Equal(testCase.want, got) {
			t.Errorf("Failed to parse \"%v\", got: %v", testCase.recipient, got)
		}
	}
}
