// internal/services/notification_service.go
package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/smtp"
	"time"

	"tagscale/internal/config"
	"tagscale/internal/models"

	"github.com/slack-go/slack"
	"gorm.io/gorm"
)

type slackSender interface {
	PostMessage(channelID, message string) error
}

type realSlackClient struct {
	client *slack.Client
}

func (r *realSlackClient) PostMessage(channelID, message string) error {
	_, _, err := r.client.PostMessage(channelID, slack.MsgOptionText(message, false))
	return err
}

type NotificationService struct {
	config      *config.Config
	db          *gorm.DB
	slackClient slackSender
	sendMail    func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

// NewNotificationService creates a new NotificationService with the provided
// configuration and database connection.
func NewNotificationService(config *config.Config, db *gorm.DB) *NotificationService {
	svc := &NotificationService{
		config:   config,
		db:       db,
		sendMail: smtp.SendMail,
	}
	if config.SlackToken != "" {
		svc.slackClient = &realSlackClient{client: slack.New(config.SlackToken)}
	}
	return svc
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
	if s.slackClient == nil {
		s.slackClient = &realSlackClient{client: slack.New(s.config.SlackToken)}
	}

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

	return s.slackClient.PostMessage(s.config.SlackChannel, message)
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
	var data struct {
		Date            time.Time
		TotalCost       float64
		UntaggedCost    float64
		UntaggedPercent float64
		TopServices     []models.CostSummary
		Insights        []string
	}
	data.Date = analysis.Date
	data.TotalCost = analysis.TotalCost
	data.UntaggedCost = analysis.UntaggedCost
	data.UntaggedPercent = analysis.UntaggedPercent
	_ = json.Unmarshal([]byte(analysis.TopServices), &data.TopServices)
	_ = json.Unmarshal([]byte(analysis.Insights), &data.Insights)

	if err := t.Execute(&buf, data); err != nil {
		return err
	}

	// Send email
	auth := smtp.PlainAuth("", s.config.EmailUsername, s.config.EmailPassword, s.config.EmailSMTPHost)

	msg := fmt.Sprintf("To: %s\r\nSubject: TagScale Daily Cost Report\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		s.config.EmailUsername, buf.String())

	return s.sendMail(
		fmt.Sprintf("%s:%d", s.config.EmailSMTPHost, s.config.EmailSMTPPort),
		auth,
		s.config.EmailUsername,
		[]string{s.config.EmailUsername},
		[]byte(msg),
	)
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
