// internal/services/notification_service.go
package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/smtp"

	"tagscale/internal/config"
	"tagscale/internal/models"

	"github.com/slack-go/slack"
	"gorm.io/gorm"
)

type NotificationService struct {
	config *config.Config
	db     *gorm.DB
}

func NewNotificationService(config *config.Config) *NotificationService {
	return &NotificationService{
		config: config,
	}
}

func (s *NotificationService) SendDailyDigest() error {
	// Get latest analysis
	var analysis models.CostAnalysis
	if err := s.db.Order("date DESC").First(&analysis).Error; err != nil {
		return fmt.Errorf("failed to get latest analysis: %w", err)
	}

	// Send to Slack if configured
	if s.config.SlackToken != "" {
		if err := s.sendSlackDigest(analysis); err != nil {
			return fmt.Errorf("failed to send Slack digest: %w", err)
		}
	}

	// Send email if configured
	if s.config.EmailSMTPHost != "" {
		if err := s.sendEmailDigest(analysis); err != nil {
			return fmt.Errorf("failed to send email digest: %w", err)
		}
	}

	return nil
}

func (s *NotificationService) sendSlackDigest(analysis models.CostAnalysis) error {
	api := slack.New(s.config.SlackToken)

	message := fmt.Sprintf(`📊 *Daily Cost Report - %s*

💰 *Total Cost:* $%.2f
🏷️ *Untagged Cost:* $%.2f (%.1f%%)

*Top Services:*
%s

*Insights:*
%s`,
		analysis.Date.Format("2006-01-02"),
		analysis.TotalCost,
		analysis.UntaggedCost,
		analysis.UntaggedPercent,
		s.formatTopServices(analysis.TopServices),
		s.formatInsights(analysis.Insights),
	)

	_, _, err := api.PostMessage(s.config.SlackChannel, slack.MsgOptionText(message, false))
	return err
}

func (s *NotificationService) sendEmailDigest(analysis models.CostAnalysis) error {
	// Email template
	tmpl := `
    <!DOCTYPE html>
    <html>
    <head>
        <title>TagScale Daily Cost Report</title>
    </head>
    <body>
        <h2>Daily Cost Report - {{.Date}}</h2>
        <p><strong>Total Cost:</strong> ${{.TotalCost}}</p>
        <p><strong>Untagged Cost:</strong> ${{.UntaggedCost}} ({{.UntaggedPercent}}%)</p>
        
        <h3>Top Services</h3>
        <ul>
        {{range .TopServices}}
            <li>{{.Service}}: ${{.TotalCost}}</li>
        {{end}}
        </ul>
        
        <h3>Insights</h3>
        <ul>
        {{range .Insights}}
            <li>{{.}}</li>
        {{end}}
        </ul>
    </body>
    </html>
    `

	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, analysis); err != nil {
		return err
	}

	// Send email
	auth := smtp.PlainAuth("", s.config.EmailUsername, s.config.EmailPassword, s.config.EmailSMTPHost)

	msg := fmt.Sprintf("To: %s\r\nSubject: TagScale Daily Cost Report\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		s.config.EmailUsername, buf.String())

	err = smtp.SendMail(
		fmt.Sprintf("%s:%d", s.config.EmailSMTPHost, s.config.EmailSMTPPort),
		auth,
		s.config.EmailUsername,
		[]string{s.config.EmailUsername},
		[]byte(msg),
	)

	return err
}

func (s *NotificationService) formatTopServices(topServicesJSON string) string {
	var services []models.CostSummary
	json.Unmarshal([]byte(topServicesJSON), &services)

	var result string
	for _, service := range services {
		result += fmt.Sprintf("• %s: $%.2f\n", service.Service, service.TotalCost)
	}
	return result
}

func (s *NotificationService) formatInsights(insightsJSON string) string {
	var insights []string
	json.Unmarshal([]byte(insightsJSON), &insights)

	var result string
	for _, insight := range insights {
		result += fmt.Sprintf("• %s\n", insight)
	}
	return result
}
