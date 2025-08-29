package services

import (
	"fmt"
	"net/smtp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"tagscale/internal/config"
	"tagscale/internal/models"
)

type mockSlackClient struct {
	channel string
	message string
	calls   int
	err     error
}

func (m *mockSlackClient) PostMessage(channel, message string) error {
	m.channel = channel
	m.message = message
	m.calls++
	return m.err
}

type mailCall struct {
	addr string
	from string
	to   []string
	msg  string
}

func setupDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CostRecord{}, &models.CostAnalysis{}, &models.TeamMapping{}))
	return db
}

func TestSendDailyDigestFormatsOutputs(t *testing.T) {
	db := setupDB(t)
	analysis := models.CostAnalysis{
		Date:            time.Date(2023, 8, 24, 0, 0, 0, 0, time.UTC),
		TotalCost:       100,
		UntaggedCost:    25,
		UntaggedPercent: 25,
		TopServices:     `[{"service":"EC2","total_cost":60},{"service":"S3","total_cost":40}]`,
		Insights:        `["Insight1","Insight2"]`,
	}
	require.NoError(t, db.Create(&analysis).Error)

	cfg := &config.Config{SlackToken: "x", SlackChannel: "#general", EmailSMTPHost: "smtp.example.com", EmailSMTPPort: 25, EmailUsername: "user@example.com", EmailPassword: "pass"}

	svc := NewNotificationService(cfg, db)
	mockSlack := &mockSlackClient{}
	svc.slackClient = mockSlack
	var sent mailCall
	svc.sendMail = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		sent = mailCall{addr: addr, from: from, to: to, msg: string(msg)}
		return nil
	}

	require.NoError(t, svc.SendDailyDigest())
	expectedSlack, err := svc.formatSlackMessage(analysis)
	require.NoError(t, err)
	require.Equal(t, 1, mockSlack.calls)
	require.Equal(t, cfg.SlackChannel, mockSlack.channel)
	require.Equal(t, expectedSlack, mockSlack.message)
	require.Contains(t, sent.msg, "<h2>Daily Cost Report - 2023-08-24")
	require.Contains(t, sent.msg, "<li>EC2: $60</li>")
	require.Contains(t, sent.msg, "Insight1")
}

func (s *NotificationService) formatSlackMessage(a models.CostAnalysis) (string, error) {
	ts, err := s.formatTopServices(a.TopServices)
	if err != nil {
		return "", err
	}
	ins, err := s.formatInsights(a.Insights)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`📊 *Daily Cost Report - %s*

💰 *Total Cost:* $%.2f
🏷️ *Untagged Cost:* $%.2f (%.1f%%)

*Top Services:*
%s

*Insights:*
%s`,
		a.Date.Format("2006-01-02"),
		a.TotalCost,
		a.UntaggedCost,
		a.UntaggedPercent,
		ts,
		ins,
	), nil
}

func TestSendDailyDigestMissingConfig(t *testing.T) {
	db := setupDB(t)
	analysis := models.CostAnalysis{Date: time.Now(), TopServices: "[]", Insights: "[]"}
	require.NoError(t, db.Create(&analysis).Error)
	cfg := &config.Config{}
	svc := NewNotificationService(cfg, db)
	mockSlack := &mockSlackClient{}
	svc.slackClient = mockSlack
	called := false
	svc.sendMail = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		called = true
		return nil
	}
	require.NoError(t, svc.SendDailyDigest())
	require.Equal(t, 0, mockSlack.calls)
	require.False(t, called)
}

func TestFormatTopServicesInvalidJSON(t *testing.T) {
	svc := &NotificationService{}
	_, err := svc.formatTopServices("invalid")
	require.Error(t, err)
}

func TestFormatInsightsInvalidJSON(t *testing.T) {
	svc := &NotificationService{}
	_, err := svc.formatInsights("invalid")
	require.Error(t, err)
}

func TestSendEmailDigestInvalidJSON(t *testing.T) {
	db := setupDB(t)
	analysis := models.CostAnalysis{
		Date:        time.Now(),
		TotalCost:   10,
		TopServices: "{invalid",
		Insights:    "[]",
	}
	cfg := &config.Config{EmailSMTPHost: "smtp.example.com", EmailSMTPPort: 25, EmailUsername: "user@example.com", EmailPassword: "pass"}
	svc := NewNotificationService(cfg, db)
	svc.sendMail = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error { return nil }
	err := svc.sendEmailDigest(analysis)
	require.Error(t, err)
}
