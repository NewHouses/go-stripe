package main

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
	"time"

	mail "github.com/xhit/go-simple-mail/v2"
)

//go:embed email-templates
var emailTemplateFS embed.FS

func (app *application) SendMail(from, to, subject, tmpl string, attachments []string, data interface{}) error {
	templateToRender := fmt.Sprintf("email-templates/%s.html.tmpl", tmpl)
	formattedMessage, err := RenderTemplate("email-html", templateToRender, data)
	if err != nil {
		app.errorLog.Println(err)
		return err
	}

	// send the mail
	server := mail.NewSMTPClient()
	server.Host = app.config.smtp.host
	server.Port = app.config.smtp.port
	server.Username = app.config.smtp.username
	server.Password = app.config.smtp.password
	server.Encryption = mail.EncryptionTLS
	server.KeepAlive = false
	server.ConnectTimeout = 10 * time.Second
	server.SendTimeout = 10 * time.Second

	app.infoLog.Printf("connecting smtp server %s on port %d", server.Host, server.Port)
	smtpClient, err := server.Connect()
	if err != nil {
		app.errorLog.Println(err)
		return err
	}
	app.infoLog.Println("smtp server connected")

	email := mail.NewMSG()
	email.SetFrom(from).
		AddTo(to).
		SetSubject(subject)

	email.SetBody(mail.TextHTML, formattedMessage)
	//email.AddAlternative(mail.TextPlain, plainMessage)
	if err != nil {
		app.errorLog.Println(err)
		return err
	}

	if len(attachments) > 0 {
		for _, x := range attachments {
			email.AddAttachment(x)
		}
	}

	app.infoLog.Println("sending email...")
	err = email.Send(smtpClient)
	if err != nil {
		app.errorLog.Println(err)
		return err
	}
	app.infoLog.Println("email sent")

	return nil
}

func RenderTemplate(templateType, templateToRender string, data interface{}) (string, error) {
	t, err := template.New(templateType).ParseFS(emailTemplateFS, templateToRender)
	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	if err = t.ExecuteTemplate(&tpl, "body", data); err != nil {
		return "", err
	}

	return tpl.String(), nil
}
