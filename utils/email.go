package utils

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"
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

// ─── Generic HTML mailer with CC ─────────────────────────────────────────────

// SendEmail sends an HTML email via Gmail SMTP.
//
//	to       — primary recipient address (To header)
//	cc       — carbon-copy addresses (CC header); pass nil to omit
//	subject  — email subject line
//	htmlBody — full HTML content
func SendEmail(to string, cc []string, subject, htmlBody string) error {
	from := os.Getenv("SMTP_EMAIL")
	password := os.Getenv("SMTP_PASSWORD")

	if from == "" || password == "" {
		return fmt.Errorf("SMTP credentials are not configured in environment")
	}

	smtpHost := "smtp.gmail.com"
	smtpPort := "587"
	auth := smtp.PlainAuth("", from, password, smtpHost)

	// Build recipient list (To + CC for the SMTP envelope)
	allRecipients := append([]string{to}, cc...)

	ccHeader := ""
	if len(cc) > 0 {
		ccHeader = "Cc: " + strings.Join(cc, ", ") + "\r\n"
	}

	msg := fmt.Sprintf(
		"From: Task Manager <%s>\r\nTo: %s\r\n%sSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n%s",
		from, to, ccHeader, subject, htmlBody,
	)

	return smtp.SendMail(smtpHost+":"+smtpPort, auth, from, allRecipients, []byte(msg))
}

// BuildReminderEmailBody returns a premium styled HTML email body for a due-date reminder.
//
//	assigneeName — display name of the task assignee
//	taskTitle    — task title
//	taskSummary  — short task summary
//	dueDate      — formatted due date string (e.g. "2026-06-01")
//	daysLeft     — human-readable days remaining (e.g. "7")
func BuildReminderEmailBody(assigneeName, taskTitle, taskSummary, dueDate, daysLeft string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
<style>
  body { margin:0; padding:0; background:#f0f4f8; font-family:'Segoe UI',Arial,sans-serif; }
  .wrapper { max-width:600px; margin:40px auto; background:#ffffff; border-radius:12px;
             box-shadow:0 4px 24px rgba(0,0,0,0.08); overflow:hidden; }
  .header  { background:linear-gradient(135deg,#6366f1,#8b5cf6); padding:36px 40px; text-align:center; }
  .header h1 { color:#ffffff; margin:0; font-size:24px; font-weight:700; letter-spacing:-0.5px; }
  .header p  { color:rgba(255,255,255,0.85); margin:8px 0 0; font-size:14px; }
  .badge   { display:inline-block; background:rgba(255,255,255,0.2); color:#fff;
             padding:4px 14px; border-radius:20px; font-size:13px; font-weight:600; margin-top:12px; }
  .body    { padding:36px 40px; }
  .greeting{ font-size:16px; color:#374151; margin:0 0 20px; }
  .card    { background:#f9fafb; border:1px solid #e5e7eb; border-radius:10px;
             padding:24px; margin-bottom:24px; }
  .card .label   { font-size:11px; font-weight:700; text-transform:uppercase;
                   letter-spacing:1px; color:#9ca3af; margin-bottom:4px; }
  .card .value   { font-size:16px; color:#111827; font-weight:600; }
  .card .summary { font-size:14px; color:#6b7280; margin-top:6px; }
  .due-row { display:flex; align-items:center; gap:12px; background:#fff7ed;
             border:1px solid #fed7aa; border-radius:10px; padding:18px 24px; margin-bottom:24px; }
  .due-icon  { font-size:28px; }
  .due-text .title { font-size:13px; font-weight:700; color:#c2410c;
                     text-transform:uppercase; letter-spacing:0.5px; }
  .due-text .date  { font-size:20px; font-weight:700; color:#ea580c; margin-top:2px; }
  .due-text .days  { font-size:13px; color:#9a3412; margin-top:2px; font-weight:500; }
  .footer  { background:#f9fafb; border-top:1px solid #e5e7eb; padding:20px 40px; text-align:center; }
  .footer p { font-size:12px; color:#9ca3af; margin:0; }
</style>
</head>
<body>
<div class="wrapper">
  <div class="header">
    <h1>&#9200; Task Due Date Reminder</h1>
    <p>You have an upcoming task that needs your attention</p>
    <span class="badge">%s days remaining</span>
  </div>
  <div class="body">
    <p class="greeting">Hi <strong>%s</strong>,</p>
    <p style="color:#6b7280;font-size:14px;margin:0 0 20px;">
      This is a friendly reminder that the following task is due soon.
      Please make sure it is completed on time.
    </p>
    <div class="card">
      <div class="label">Task</div>
      <div class="value">%s</div>
      <div class="summary">%s</div>
    </div>
    <div class="due-row">
      <span class="due-icon">&#128197;</span>
      <div class="due-text">
        <div class="title">Due Date</div>
        <div class="date">%s</div>
        <div class="days">%s days left &#8212; act now!</div>
      </div>
    </div>
    <p style="font-size:13px;color:#9ca3af;margin:0;">
      If this task is already completed, you can safely ignore this email.
    </p>
  </div>
  <div class="footer">
    <p>This is an automated message from your Task Management System.<br/>
    Please do not reply to this email.</p>
  </div>
</div>
</body>
</html>`, daysLeft, assigneeName, taskTitle, taskSummary, dueDate, daysLeft)
}
