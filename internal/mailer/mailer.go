package mailer

import (
	"bytes"
	"embed"
	"github.com/go-mail/mail/v2"
	"html/template"
	"time"
)

// Declare a new variable with type embed.FS to hold our email templates
var (
	//go:embed "templates"
	templateFS embed.FS
)

// Define a Mailer struct which contains a mail.Dialer instance(used to connect to a SMTP server)
// And the name and address you want the email to be from(sender)
type Mailer struct {
	dialer *mail.Dialer
	sender string
}

func New(host string, port int, username, password, sender string) Mailer {
	// Initialize a new mail.Dialer instance with the given SMTP server settings
	// 这是一个SMTP连接拨号器，通过拨号器连接SMTP服务器
	dialer := mail.NewDialer(host, port, username, password)
	dialer.Timeout = 5 * time.Second

	// Return a Mailer instance
	return Mailer{
		dialer: dialer,
		sender: sender,
	}
}

// Send() takes the recipient email address as the first p,the name of file containing the templates,
// and any dynamic data for the templates as an interface{} p
func (m Mailer) Send(recipient, templateFile string, data interface{}) error {
	// Use the ParseFS() to parse the required template file from the embedded file system
	tmpl, err := template.New("email").ParseFS(templateFS, "templates/"+templateFile)
	if err != nil {
		return err
	}
	// Execute the named template "subject",passing in the dynamic data and storing the result
	// in a bytes.Buffer
	subject := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(subject, "subject", data)
	if err != nil {
		return err
	}

	plainBody := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(plainBody, "plainBody", data)
	if err != nil {
		return err
	}

	htmlBody := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(htmlBody, "htmlBody", data)
	if err != nil {
		return err
	}

	//
	msg := mail.NewMessage()
	msg.SetHeader("To", recipient)
	msg.SetHeader("From", m.sender)
	msg.SetHeader("Subject", subject.String())
	msg.SetBody("text/plain", plainBody.String())
	msg.AddAlternative("text/html", htmlBody.String())

	// 尝试发送三次
	for i := 1; i <= 3; i++ {
		// Call the DialAndSend() on the dialer,this opens a connection to SMTP server,sends the message
		// then closes the connection
		err = m.dialer.DialAndSend(msg)
		// 如果发送成功
		if nil == err {
			return nil
		}
		// If it didn't work, sleep for a short time and retry
		time.Sleep(500 * time.Millisecond)
	}

	return err
}
