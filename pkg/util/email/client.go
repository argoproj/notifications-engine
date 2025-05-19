// Package email is the email client which sends emails.
//
// This file has been extracted from gomodules.xyz/notify v0.1.1, and copied pretty much verbatim. The only difference
// between this file and the original is that we add the ability to use custom certificates for the SMTP server in the
// same manner as done in the http package.
package email

import (
	"errors"

	"github.com/argoproj/notifications-engine/pkg/util/http"
	"gopkg.in/gomail.v2"
)

const UID = "smtp"

type ByEmail interface {
	UID() string
	From(from string) ByEmail
	WithSubject(subject string) ByEmail
	WithBody(body string) ByEmail
	WithTag(tag string) ByEmail
	WithNoTracking() ByEmail
	To(to string, cc ...string) ByEmail
	Send() error
	SendHtml() error
}

type Options struct {
	Host               string   `json:"host" envconfig:"HOST" required:"true" form:"smtp_host"`
	Port               int      `json:"port" envconfig:"PORT" required:"true" form:"smtp_port"`
	InsecureSkipVerify bool     `json:"insecure_skip_verify" envconfig:"INSECURE_SKIP_VERIFY" form:"smtp_insecure_skip_verify"`
	Username           string   `json:"username" envconfig:"USERNAME" required:"true" form:"smtp_username"`
	Password           string   `json:"password" envconfig:"PASSWORD" required:"true" form:"smtp_password"`
	From               string   `json:"from" envconfig:"FROM" required:"true" form:"smtp_from"`
	To                 []string `json:"to" envconfig:"TO" form:"smtp_to"`
}

type emailClient struct {
	opt     Options
	subject string
	body    string
	html    bool
}

func New(opt Options) ByEmail {
	return &emailClient{opt: opt}
}

func (c emailClient) UID() string {
	return UID
}

func (c emailClient) From(from string) ByEmail {
	c.opt.From = from
	return &c
}

func (c emailClient) WithSubject(subject string) ByEmail {
	c.subject = subject
	return &c
}

func (c emailClient) WithBody(body string) ByEmail {
	c.body = body
	return &c
}

func (c emailClient) WithTag(tag string) ByEmail {
	return &c
}

func (c emailClient) WithNoTracking() ByEmail {
	return &c
}

func (c emailClient) To(to string, cc ...string) ByEmail {
	c.opt.To = append([]string{to}, cc...)
	return &c
}

func (c *emailClient) Send() error {
	if len(c.opt.To) == 0 {
		return errors.New("Missing to")
	}

	mail := gomail.NewMessage()
	mail.SetHeader("From", c.opt.From)
	mail.SetHeader("To", c.opt.To...)
	mail.SetHeader("Subject", c.subject)
	if c.html {
		mail.SetBody("text/html", c.body)
	} else {
		mail.SetBody("text/plain", c.body)
	}

	var d *gomail.Dialer
	if c.opt.Username != "" && c.opt.Password != "" {
		d = gomail.NewDialer(c.opt.Host, c.opt.Port, c.opt.Username, c.opt.Password)
	} else {
		d = &gomail.Dialer{Host: c.opt.Host, Port: c.opt.Port}
	}

	config, err := http.GetTLSConfigFromHost(c.opt.Host, c.opt.InsecureSkipVerify)
	if err != nil {
		return err
	}

	d.TLSConfig = config

	return d.DialAndSend(mail)
}

func (c emailClient) SendHtml() error {
	c.html = true
	return c.Send()
}
