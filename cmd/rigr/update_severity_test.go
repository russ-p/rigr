package main

import "testing"

func TestClassifySeverity_DefaultWhenEmpty(t *testing.T) {
	if got := ClassifySeverity(""); got != UpdateSeverityDefault {
		t.Fatalf("expected default, got %q", got)
	}
}

func TestClassifySeverity_CaseInsensitive(t *testing.T) {
	if got := ClassifySeverity("This fixes CVE-2026-1234"); got != UpdateSeveritySecurityFixes {
		t.Fatalf("expected security_fixes, got %q", got)
	}
}

func TestClassifySeverity_SecurityWins(t *testing.T) {
	text := "BREAKING CHANGES: API is incompatible. Also security fix for vulnerability."
	if got := ClassifySeverity(text); got != UpdateSeveritySecurityFixes {
		t.Fatalf("expected security_fixes, got %q", got)
	}
}

func TestSeverityEmoji(t *testing.T) {
	if got := SeverityEmoji(UpdateSeveritySecurityFixes); got == "" {
		t.Fatalf("expected non-empty emoji for security_fixes")
	}
	if got := SeverityEmoji(UpdateSeverityBreakingChanges); got == "" {
		t.Fatalf("expected non-empty emoji for breaking_changes")
	}
	if got := SeverityEmoji(UpdateSeverityDefault); got != "" {
		t.Fatalf("expected empty emoji for default, got %q", got)
	}
}

