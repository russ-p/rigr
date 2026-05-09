package main

import "strings"

type UpdateSeverity string

const (
	UpdateSeverityDefault         UpdateSeverity = "default"
	UpdateSeverityBreakingChanges UpdateSeverity = "breaking_changes"
	UpdateSeveritySecurityFixes   UpdateSeverity = "security_fixes"
)

func ClassifySeverity(text string) UpdateSeverity {
	s := strings.ToLower(strings.TrimSpace(text))
	if s == "" {
		return UpdateSeverityDefault
	}

	// Priority: security > breaking > default
	if containsAny(s,
		"security",
		"cve",
		"vuln",
		"vulnerability",
	) {
		return UpdateSeveritySecurityFixes
	}

	if containsAny(s,
		"breaking change",
		"breaking changes",
		"backward incompatible",
		"incompatible",
	) {
		return UpdateSeverityBreakingChanges
	}

	return UpdateSeverityDefault
}

func SeverityEmoji(sev UpdateSeverity) string {
	switch sev {
	case UpdateSeveritySecurityFixes:
		return "🔒"
	case UpdateSeverityBreakingChanges:
		return "💥"
	default:
		return ""
	}
}

func MaxSeverity(a, b UpdateSeverity) UpdateSeverity {
	if severityRank(a) >= severityRank(b) {
		return a
	}
	return b
}

func severityRank(sev UpdateSeverity) int {
	switch sev {
	case UpdateSeveritySecurityFixes:
		return 2
	case UpdateSeverityBreakingChanges:
		return 1
	default:
		return 0
	}
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if n == "" {
			continue
		}
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

