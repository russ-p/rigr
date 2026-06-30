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

func TestShouldSkipFeedItem_DefaultPreReleaseSuffixes(t *testing.T) {
	skipRe := regexp.MustCompile(defaultSkipVersionRegex)
	cases := []struct {
		title string
		link  string
		want  bool
	}{
		{title: "v1.2.3-dev", want: true},
		{title: "v1.2.3-rc", want: true},
		{title: "v1.2.3-rc1", want: true},
		{title: "2026.7.0b2", want: true},
		{title: "v1.2.3", want: false},
		{link: "https://example/releases/v2.0.0-rc2", want: true},
	}
	for _, tc := range cases {
		got := ShouldSkipFeedItem(skipRe, &gofeed.Item{Title: tc.title, Link: tc.link})
		if got != tc.want {
			t.Fatalf("title=%q link=%q: expected skip=%v, got %v", tc.title, tc.link, tc.want, got)
		}
	}
}

func TestFilterSkippedFeedItems_ExcludesPreReleasesFromMatching(t *testing.T) {
	feedRe := regexp.MustCompile(defaultFeedVersionRegex)
	skipRe := regexp.MustCompile(defaultSkipVersionRegex)
	items := []*gofeed.Item{
		{Title: "v1.2.3-rc1", Link: "https://example/releases/v1.2.3-rc1"},
		{Title: "v1.2.3", Link: "https://example/releases/v1.2.3"},
		{Title: "v1.2.0", Link: "https://example/releases/v1.2.0"},
	}

	filtered := FilterSkippedFeedItems(items, skipRe)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 items after filter, got %d", len(filtered))
	}
	if filtered[0].Title != "v1.2.3" {
		t.Fatalf("expected latest stable %q, got %q", "v1.2.3", filtered[0].Title)
	}

	idx := FindMatchingFeedIndex(filtered, feedRe, "1.2.0")
	if idx != 1 {
		t.Fatalf("expected idx %d, got %d", 1, idx)
	}
}

func TestCompileSkipVersionRegex_DisableSentinel(t *testing.T) {
	if got := CompileSkipVersionRegex(nil, "-"); got != nil {
		t.Fatalf("expected nil regex for disable sentinel, got %#v", got)
	}
}

