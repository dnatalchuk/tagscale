package models

import (
	"time"
)

type CostRecord struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Date       time.Time `json:"date" gorm:"index"`
	Service    string    `json:"service" gorm:"index"`
	Account    string    `json:"account" gorm:"index"`
	Region     string    `json:"region" gorm:"index"`
	Cost       float64   `json:"cost"`
	Currency   string    `json:"currency"`
	Tags       string    `json:"tags"` // JSON string
	ResourceID string    `json:"resource_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CostAnalysis struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	Date            time.Time `json:"date" gorm:"index"`
	TotalCost       float64   `json:"total_cost"`
	UntaggedCost    float64   `json:"untagged_cost"`
	UntaggedPercent float64   `json:"untagged_percent"`
	TopServices     string    `json:"top_services"` // JSON string
	TopAccounts     string    `json:"top_accounts"` // JSON string
	TopRegions      string    `json:"top_regions"`  // JSON string
	CostByTeam      string    `json:"cost_by_team"` // JSON string
	Insights        string    `json:"insights"`     // JSON string
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type TeamMapping struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Pattern     string    `json:"pattern" gorm:"index"`
	PatternType string    `json:"pattern_type"` // "tag", "resource_name", "service"
	Team        string    `json:"team"`
	Priority    int       `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CostSummary struct {
	Service    string  `json:"service"`
	Account    string  `json:"account"`
	Region     string  `json:"region"`
	Team       string  `json:"team"`
	TotalCost  float64 `json:"total_cost"`
	Percentage float64 `json:"percentage"`
}

type TagInference struct {
	ResourcePattern string            `json:"resource_pattern"`
	InferredTags    map[string]string `json:"inferred_tags"`
	Confidence      float64           `json:"confidence"`
}
