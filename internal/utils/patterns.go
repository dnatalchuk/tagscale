package utils

import (
	"regexp"
	"strings"
)

// teamPatternStrings holds raw patterns for inferring team ownership.
var teamPatternStrings = map[string][]string{
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

// TeamPatterns stores compiled regular expressions for team inference.
var TeamPatterns map[string][]*regexp.Regexp

func init() {
	TeamPatterns = make(map[string][]*regexp.Regexp, len(teamPatternStrings))
	for team, patterns := range teamPatternStrings {
		for _, p := range patterns {
			TeamPatterns[team] = append(TeamPatterns[team], regexp.MustCompile(p))
		}
	}
}

func InferTeamFromResourceName(resourceName string) string {
	resourceName = strings.ToLower(resourceName)

	for team, patterns := range TeamPatterns {
		for _, pattern := range patterns {
			if pattern.MatchString(resourceName) {
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
