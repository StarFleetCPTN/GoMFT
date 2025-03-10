package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"time"

	"github.com/starfleetcptn/gomft/internal/config"
)

// Service represents the email sending service
type Service struct {
	Config *config.Config
}

// NewService creates a new email service
func NewService(cfg *config.Config) *Service {
	return &Service{
		Config: cfg,
	}
}

// SendPasswordResetEmail sends a password reset email to the specified email address
func (s *Service) SendPasswordResetEmail(toEmail, username, resetToken string) error {
	if !s.Config.Email.Enabled {
		// If email is not enabled, just log it (you can redirect to the default logging logic)
		return fmt.Errorf("email service is disabled, reset link would be: %s/reset-password?token=%s",
			s.Config.BaseURL, resetToken)
	}

	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.Config.BaseURL, resetToken)

	// Create email data for template
	data := map[string]interface{}{
		"Username":     username,
		"ResetLink":    resetLink,
		"AppName":      "GoMFT",
		"Year":         time.Now().Year(),
		"ExpiresHours": 0.25, // Token expiration time in hours (15 minutes = 0.25 hours)
	}

	// Generate email content
	subject := "Password Reset Request - GoMFT"
	htmlContent, err := s.generatePasswordResetEmailHTML(data)
	if err != nil {
		return err
	}

	// Send the email
	return s.sendEmail(toEmail, subject, htmlContent)
}

// generatePasswordResetEmailHTML generates the HTML content for password reset emails
func (s *Service) generatePasswordResetEmailHTML(data map[string]interface{}) (string, error) {
	// HTML template for password reset email
	tmpl, err := template.New("passwordResetEmail").Parse(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Reset Your Password</title>
    <style>
        /* Base styles */
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            margin: 0;
            padding: 0;
            background-color: #f9fafb;
            color: #374151;
            line-height: 1.5;
        }
        .container {
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
            background-color: #ffffff;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);
        }
        .header {
            text-align: center;
            padding: 20px 0;
            border-bottom: 1px solid #e5e7eb;
        }
        .logo {
            width: 60px;
            height: 60px;
            margin: 0 auto 15px;
            background-color: #2563eb;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .logo-icon {
            font-size: 24px;
            color: white;
            font-weight: bold;
        }
        h1 {
            color: #111827;
            font-size: 24px;
            margin: 0;
        }
        .content {
            padding: 30px 20px;
        }
        p {
            margin: 0 0 15px;
            color: #4b5563;
        }
        .btn {
            display: inline-block;
            background-color: #2563eb;
            color: #ffffff;
            text-decoration: none;
            padding: 12px 30px;
            border-radius: 6px;
            font-weight: 600;
            margin: 25px 0;
            text-align: center;
        }
        .btn:hover {
            background-color:rgb(55, 113, 236);
        }
        .reset-link {
            margin: 20px 0;
            padding: 15px;
            background-color: #f3f4f6;
            border-radius: 6px;
            word-break: break-all;
            font-family: monospace;
            color: #4b5563;
        }
        .note {
            font-size: 14px;
            color: #6b7280;
            margin-top: 30px;
            padding-top: 15px;
            border-top: 1px solid #e5e7eb;
        }
        .footer {
            text-align: center;
            font-size: 12px;
            color: #9ca3af;
            padding: 20px 0;
            background-color: #f9fafb;
            border-radius: 0 0 8px 8px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">
                <div class="logo-icon">G</div>
            </div>
            <h1>Reset Your Password</h1>
        </div>
        <div class="content">
            <p>Hello{{if .Username}} {{.Username}}{{end}},</p>
            <p>We received a request to reset your password for your {{.AppName}} account. Click the button below to reset it:</p>
            
            <div style="text-align: center;">
                <a href="{{.ResetLink}}" class="btn">Reset Password</a>
            </div>
            
            <p>If the button doesn't work, you can copy and paste the following link into your browser:</p>
            <div class="reset-link">{{.ResetLink}}</div>
            
            <p>This link will expire in 15 minutes.</p>
            
            <div class="note">
                <p>If you didn't request a password reset, you can ignore this email. Your password will remain unchanged.</p>
            </div>
        </div>
        <div class="footer">
            <p>&copy; {{.Year}} {{.AppName}}. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`)
	if err != nil {
		return "", err
	}

	var result bytes.Buffer
	if err := tmpl.Execute(&result, data); err != nil {
		return "", err
	}

	return result.String(), nil
}

// sendEmail sends an email with the given subject and HTML content
func (s *Service) sendEmail(toEmail, subject, htmlContent string) error {
	from := s.Config.Email.FromEmail
	if s.Config.Email.FromName != "" {
		from = fmt.Sprintf("%s <%s>", s.Config.Email.FromName, s.Config.Email.FromEmail)
	}

	// Construct email headers
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = toEmail
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	if s.Config.Email.ReplyTo != "" {
		headers["Reply-To"] = s.Config.Email.ReplyTo
	}

	// Construct email message
	message := ""
	for key, value := range headers {
		message += fmt.Sprintf("%s: %s\r\n", key, value)
	}
	message += "\r\n" + htmlContent

	// Set up the SMTP server address
	addr := fmt.Sprintf("%s:%d", s.Config.Email.Host, s.Config.Email.Port)

	// Check if authentication is required
	if s.Config.Email.RequireAuth {
		// Use authenticated SMTP
		auth := smtp.PlainAuth("", s.Config.Email.Username, s.Config.Email.Password, s.Config.Email.Host)
		return smtp.SendMail(addr, auth, s.Config.Email.FromEmail, []string{toEmail}, []byte(message))
	} else {
		// Use unauthenticated SMTP
		client, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server: %v", err)
		}
		defer client.Close()

		// Set up TLS if enabled
		if s.Config.Email.EnableTLS {
			if err := client.StartTLS(nil); err != nil {
				return fmt.Errorf("failed to start TLS: %v", err)
			}
		}

		// Set the sender and recipient
		if err := client.Mail(s.Config.Email.FromEmail); err != nil {
			return fmt.Errorf("failed to set sender: %v", err)
		}
		if err := client.Rcpt(toEmail); err != nil {
			return fmt.Errorf("failed to set recipient: %v", err)
		}

		// Send the email body
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("failed to get data writer: %v", err)
		}
		_, err = w.Write([]byte(message))
		if err != nil {
			return fmt.Errorf("failed to write email data: %v", err)
		}
		err = w.Close()
		if err != nil {
			return fmt.Errorf("failed to close data writer: %v", err)
		}

		return client.Quit()
	}
}
