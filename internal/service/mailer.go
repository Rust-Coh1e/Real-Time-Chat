package service

import (
	"net/smtp"
	"real-time-chat/config"
)

type Mailer struct {
	host string
	port string
	from string
}

func NewMailer(cfg config.Config) *Mailer {
	return &Mailer{
		host: cfg.MailerHost,
		port: cfg.MailerPort,
		from: cfg.MailerFrom,
	}
}

func (mail *Mailer) SendVerification(userMail string, userName string, tokenURL string) error {

	subject := "Verify your account"
	body := "Hi, " + userName + "!\n Verify your email\r\n\r\nClick here: " + tokenURL
	message := []byte("Subject: " + subject + "\n\n" + body)

	return smtp.SendMail(mail.host+":"+mail.port, nil, mail.from, []string{userMail}, message)
}
