package main

import (
	"regexp"
	"testing"

	"github.com/mmcdole/gofeed"
)

func TestExtractVersion_DefaultSemverCore_ImageVariantStripped(t *testing.T) {
	re := regexp.MustCompile(defaultImageVersionRegex)
	got := ExtractVersion(re, "26.5.0-alpine")
	if got != "26.5.0" {
		t.Fatalf("expected %q, got %q", "26.5.0", got)
	}
}

func TestExtractVersion_DefaultFeed_VPrefixStripped(t *testing.T) {
	re := regexp.MustCompile(defaultFeedVersionRegex)
	got := ExtractVersion(re, "v26.5.2")
	if got != "26.5.2" {
		t.Fatalf("expected %q, got %q", "26.5.2", got)
	}
}

func TestFindMatchingFeedIndex_MatchesCurrentVariantAgainstFeed(t *testing.T) {
	feedRe := regexp.MustCompile(defaultFeedVersionRegex)
	items := []*gofeed.Item{
		{Title: "v26.5.2", Link: "https://example/releases/v26.5.2"},
		{Title: "v26.5.1", Link: "https://example/releases/v26.5.1"},
		{Title: "v26.5.0", Link: "https://example/releases/v26.5.0"},
	}

	cur := ExtractVersion(regexp.MustCompile(defaultImageVersionRegex), "26.5.0-alpine")
	idx := FindMatchingFeedIndex(items, feedRe, cur)
	if idx != 2 {
		t.Fatalf("expected idx %d, got %d", 2, idx)
	}
}

func TestExtractVersion_LabelOverride_UsesCaptureGroup1(t *testing.T) {
	re := regexp.MustCompile(`(?i)^release-(\d+\.\d+\.\d+)$`)
	got := ExtractVersion(re, "release-1.2.3")
	if got != "1.2.3" {
		t.Fatalf("expected %q, got %q", "1.2.3", got)
	}
}

func TestExtractFeedVersion_FallsBackToLink(t *testing.T) {
	re := regexp.MustCompile(defaultFeedVersionRegex)
	it := &gofeed.Item{
		Title: "not a version",
		Link:  "https://example/releases/v9.8.7",
	}
	got := ExtractFeedVersion(re, it)
	if got != "9.8.7" {
		t.Fatalf("expected %q, got %q", "9.8.7", got)
	}
}

