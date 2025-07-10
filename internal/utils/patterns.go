package utils

import (
	"regexp"
	"strings"
)

// Common patterns for inferring team ownership
var TeamPatterns = map[string][]string{
	"frontend": {
		"^web-.*",
		"^ui-.*",
		"^frontend-.*",
		"cloudfront",
		"s3.*-web",
	},
	"backend": {
		"^api-.*",
		"^backend-.*",
		"^service-.*",
		"lambda",
		"rds",
		"elasticache",
	},
	"data": {
		"^data-.*",
		"^etl-.*",
		"^analytics-.*",
		"redshift",
		"kinesis",
		"glue",
		"emr",
	},
	"devops": {
		"^infra-.*",
		"^ops-.*",
		"^monitoring-.*",
		"cloudwatch",
		"cloudtrail",
		"config",
	},
	"mobile": {
		"^mobile-.*",
		"^app-.*",
		"^ios-.*",
		"^android-.*",
		"cognito",
		"pinpoint",
	},
}

func InferTeamFromResourceName(resourceName string) string {
	resourceName = strings.ToLower(resourceName)

	for team, patterns := range TeamPatterns {
		for _, pattern := range patterns {
			matched, _ := regexp.MatchString(pattern, resourceName)
			if matched {
				return team
			}
		}
	}

	return "unassigned"
}

func InferTeamFromService(service string) string {
	service = strings.ToLower(service)

	serviceToTeam := map[string]string{
		"amazon cloudfront":  "frontend",
		"amazon s3":          "backend",
		"aws lambda":         "backend",
		"amazon rds":         "backend",
		"amazon elasticache": "backend",
		"amazon redshift":    "data",
		"amazon kinesis":     "data",
		"aws glue":           "data",
		"amazon emr":         "data",
		"amazon cloudwatch":  "devops",
		"aws cloudtrail":     "devops",
		"aws config":         "devops",
		"amazon cognito":     "mobile",
		"amazon pinpoint":    "mobile",
	}

	if team, exists := serviceToTeam[service]; exists {
		return team
	}

	return "unassigned"
}
