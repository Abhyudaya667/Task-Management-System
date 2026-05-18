package utils

import (
	"fmt"
	"net/smtp"
	"os"
)

// SendVerificationEmail sends an email with a verification link containing the token.
func SendVerificationEmail(toEmail, username, token string) error {
	smtpEmail := os.Getenv("SMTP_EMAIL")
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	appURL := os.Getenv("APP_URL")

	if smtpEmail == "" || smtpPassword == "" {
		return fmt.Errorf("SMTP credentials are not configured in environment")
	}

	if appURL == "" {
		appURL = "http://localhost:8080"
	}

	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	// Create the verification link (points to the backend route we will create)
	verificationLink := fmt.Sprintf("%s/auth/verify-email?token=%s", appURL, token)

	// Compose the email (using HTML for a nice clickable link)
	subject := "Subject: Verify your Task Management System account\r\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	body := fmt.Sprintf(`
		<html>
			<body>
				<h2>Hello %s,</h2>
				<p>Thank you for registering. Please verify your email address by clicking the link below:</p>
				<p><a href="%s" style="padding: 10px 15px; background-color: #007bff; color: #fff; text-decoration: none; border-radius: 5px;">Verify Email</a></p>
				<p>If you did not request this, please ignore this email.</p>
			</body>
		</html>
	`, username, verificationLink)

	msg := []byte(subject + mime + body)

	// Authentication
	auth := smtp.PlainAuth("", smtpEmail, smtpPassword, smtpHost)

	// Send Email
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, smtpEmail, []string{toEmail}, msg)
	return err
}
