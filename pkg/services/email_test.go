package services

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"gomodules.xyz/notify"
)

func TestGetTemplater_Email(t *testing.T) {
	n := Notification{
		Email: &EmailNotification{
			Subject: "{{.foo}}", Body: "{{.bar}}",
		},
	}

	templater, err := n.GetTemplater("", template.FuncMap{})
	if !assert.NoError(t, err) {
		return
	}

	var notification Notification

	err = templater(&notification, map[string]interface{}{
		"foo": "hello",
		"bar": "world",
	})

	if !assert.NoError(t, err) {
		return
	}

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
	if es.html != true {
		t.Error("Html set incorrectly")
	}
}
