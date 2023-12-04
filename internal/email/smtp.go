package email

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

type SmtpServer struct {
	HostPort string
	Tls      *tls.Config
	User     string
	Password string
	Hello    string
}

func new(options SmtpServer) (*smtp.Client, error) {
	var client *smtp.Client = nil
	var err error

	if options.Tls != nil {
		client, err = smtp.DialTLS(options.HostPort, options.Tls)
	} else {
		client, err = smtp.Dial(options.HostPort)
	}
	if err != nil {
		return nil, fmt.Errorf("could not connect to smtp server: %w", err)
	}

	if options.Hello != "" {
		if err = client.Hello(options.Hello); err != nil {
			client.Close()
			return nil, fmt.Errorf("could not greet upstream: %w", err)
		}
	}

	if options.User != "" || options.Password != "" {
		if err := client.Auth(sasl.NewLoginClient(options.User, options.Password)); err != nil {
			return nil, fmt.Errorf("AUTH failed: %w", err)
		}
	}

	return client, nil
}

func Send(options SmtpServer, email Message) error {
	client, err := new(options)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.Mail(email.From, nil); err != nil {
		return fmt.Errorf("smtp server rejected mail from '%s': %w", email.From, err)
	}

	for _, address := range email.To {
		if err := client.Rcpt(address, nil); err != nil {
			return fmt.Errorf("smtp server rejected mail to '%s': %w", address, err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp server rejected request to send mail data: %w", err)
	}
	defer writer.Close()

	err = email.Write(writer)
	if err != nil {
		return err
	}
	err = client.Quit()
	if err != nil {
		smtpError := &smtp.SMTPError{}
		if errors.As(err, &smtpError) {
			// Seems some SMTP servers return 250 instead of 221 on QUIT
			if smtpError.Code == 250 {
				return nil
			}
		}
		return err
	}
	return nil
}
