package email

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"project/database"
)

// SendEmail sends an email to the specified recipient with the given subject and body
func SendEmail(recipient, subject, body string) error {
	var SMTPHost = database.GetEnv("SMTP_HOST", "smtp.gmail.com")
	var SMTPPort = database.GetEnv("SMTP_PORT", "587")
	var SMTPEmail = database.GetEnv("SMTP_EMAIL", "")
	var SMTPPassword = database.GetEnv("SMTP_PASSWORD", "")

	// Set up authentication information for the SMTP server
	auth := smtp.PlainAuth("", SMTPEmail, SMTPPassword, SMTPHost)

	serverAddr := net.JoinHostPort(SMTPHost, SMTPPort)
	var (
		client *smtp.Client
		err    error
	)

	// Connect with implicit TLS for port 465, otherwise use a plain connection
	// and upgrade with STARTTLS when the server supports it.
	if SMTPPort == "465" {
		conn, err := tls.Dial("tcp", serverAddr, &tls.Config{ServerName: SMTPHost})
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server over TLS: %v", err)
		}
		client, err = smtp.NewClient(conn, SMTPHost)
		if err != nil {
			_ = conn.Close()
			return fmt.Errorf("failed to create SMTP client: %v", err)
		}
	} else {
		client, err = smtp.Dial(serverAddr)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server: %v", err)
		}
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: SMTPHost}); err != nil {
				_ = client.Close()
				return fmt.Errorf("failed to start TLS with SMTP server: %v", err)
			}
		} else if SMTPPassword != "" {
			_ = client.Close()
			return fmt.Errorf("SMTP server does not support STARTTLS; refusing to send credentials over an unencrypted connection")
		}
	}
	defer client.Quit()

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("failed to authenticate with SMTP server: %v", err)
	}

	//Set the sender and recipient
	if err := client.Mail(SMTPEmail); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}
	if err := client.Rcpt(recipient); err != nil {
		return fmt.Errorf("failed to set recipient: %v", err)
	}

	//Send the email body
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to create email data writer: %v", err)
	}
	defer writer.Close()

	//Write the email content
	emailContent := fmt.Sprintf("Subject: %s\r\n\r\n%s", subject, body)
	if _, err := writer.Write([]byte(emailContent)); err != nil {
		return fmt.Errorf("failed to write email content: %v", err)
	}

	return nil
}

// Email format for notifying RE of pending payment
func NotifyPendingPayment(toEmail string, permitRequestID uint) error {
	subject := "Payment Pending for Environmental Permit Request"
	body := fmt.Sprintf("Your environmental permit request with ID %d is pending payment. Please complete the payment to proceed with the review process.", permitRequestID)
	return SendEmail(toEmail, subject, body)
}

// Email format for notifying RE of payment review
func NotifyReviewingPayment(toEmail string, permitRequestID uint) error {
	subject := "Payment Under Review for Environmental Permit Request"
	body := fmt.Sprintf("Your payment for environmental permit request with ID %d is currently under review. We will notify you once the review is complete.", permitRequestID)
	return SendEmail(toEmail, subject, body)
}

// Email format for notifying RE of payment decision
func NotifyPaymentDecision(toEmail string, permitRequestID uint, decision string) error {
	subject := "Payment Decision for Environmental Permit Request"
	body := fmt.Sprintf("Your payment for environmental permit request with ID %d has been reviewed. The decision is: %s.", permitRequestID, decision)
	return SendEmail(toEmail, subject, body)
}

// Email format for notifying RE of permit being reviewed
func NotifyBeingReviewed(toEmail string, permitRequestID uint) error {
	subject := "Environmental Permit Request Being Reviewed"
	body := fmt.Sprintf("Your environmental permit request with ID %d is now being reviewed by an environmental officer. We will notify you once a decision has been made.", permitRequestID)
	return SendEmail(toEmail, subject, body)
}

// Email format for notifying RE of final permit decision
func NotifyFinalDecision(toEmail string, permitRequestID uint, decision string) error {
	subject := "Final Decision for Environmental Permit Request"
	body := fmt.Sprintf("Your environmental permit request with ID %d has received a final decision. The decision is: %s.", permitRequestID, decision)
	return SendEmail(toEmail, subject, body)
}
