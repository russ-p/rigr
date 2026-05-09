package main

import (
	"testing"
	"time"
)

func TestBuildHomepageUpdates_FiltersAndSorts(t *testing.T) {
	t1 := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)

	apps := []AppState{
		{
			ContainerName:       "b-app",
			CurrentVersion:      "1.0.0",
			CurrentMatchVersion: "1.0.0",
			UpdatesAvailable: []AppUpdate{
				{Title: "1.1.0", ReleaseNotesURL: "https://example.com/b/1.1.0", PublishedAt: &t1},
			},
			LatestKnownRelease: &AppUpdate{Title: "1.1.0", ReleaseNotesURL: "https://example.com/b/1.1.0", PublishedAt: &t1},
			LatestMatchVersion: "1.1.0",
		},
		{
			ContainerName:       "a-app",
			CurrentVersion:      "2.0.0",
			CurrentMatchVersion: "2.0.0",
			UpdatesAvailable: []AppUpdate{
				{Title: "2.1.0", ReleaseNotesURL: "https://example.com/a/2.1.0", PublishedAt: &t2, Severity: UpdateSeverityBreakingChanges},
			},
			LatestKnownRelease: &AppUpdate{Title: "2.1.0", ReleaseNotesURL: "https://example.com/a/2.1.0", PublishedAt: &t2},
			LatestMatchVersion: "2.1.0",
		},
		{
			// No updates -> excluded
			ContainerName:      "no-updates",
			CurrentVersion:     "3.0.0",
			UpdatesAvailable:   nil,
			LatestKnownRelease: &AppUpdate{Title: "3.0.0", ReleaseNotesURL: "https://example.com/n/3.0.0", PublishedAt: &t2},
		},
	}

	out := BuildHomepageUpdates(apps)
	if len(out) != 2 {
		t.Fatalf("expected 2 items, got %d", len(out))
	}

	// Sort newest first: a-app (t2), then b-app (t1)
	if out[0].ContainerName != "a-app" {
		t.Fatalf("expected first item a-app, got %q", out[0].ContainerName)
	}
	if out[1].ContainerName != "b-app" {
		t.Fatalf("expected second item b-app, got %q", out[1].ContainerName)
	}

	if out[0].VersionLine != "💥 2.0.0 \u2192 2.1.0" {
		t.Fatalf("unexpected version_line: %q", out[0].VersionLine)
	}
	if out[0].ChangelogURL != "https://example.com/a/2.1.0" {
		t.Fatalf("unexpected changelog_url: %q", out[0].ChangelogURL)
	}
	if out[0].Severity != UpdateSeverityBreakingChanges {
		t.Fatalf("unexpected severity: %q", out[0].Severity)
	}
}

func TestBuildHomepageUpdates_SortsNilPublishedAtLast(t *testing.T) {
	apps := []AppState{
		{
			ContainerName:       "with-time",
			CurrentVersion:      "1.0.0",
			CurrentMatchVersion: "1.0.0",
			UpdatesAvailable: []AppUpdate{
				{Title: "1.1.0", ReleaseNotesURL: "https://example.com/with-time"},
			},
			LatestKnownRelease: &AppUpdate{
				Title:           "1.1.0",
				ReleaseNotesURL: "https://example.com/with-time",
				PublishedAt:     ptrTime(time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)),
			},
			LatestMatchVersion: "1.1.0",
		},
		{
			ContainerName:       "no-time",
			CurrentVersion:      "1.0.0",
			CurrentMatchVersion: "1.0.0",
			UpdatesAvailable: []AppUpdate{
				{Title: "1.1.0", ReleaseNotesURL: "https://example.com/no-time"},
			},
			LatestKnownRelease: &AppUpdate{Title: "1.1.0", ReleaseNotesURL: "https://example.com/no-time", PublishedAt: nil},
			LatestMatchVersion: "1.1.0",
		},
	}

	out := BuildHomepageUpdates(apps)
	if len(out) != 2 {
		t.Fatalf("expected 2 items, got %d", len(out))
	}
	if out[0].ContainerName != "with-time" {
		t.Fatalf("expected with-time first, got %q", out[0].ContainerName)
	}
	if out[1].ContainerName != "no-time" {
		t.Fatalf("expected no-time second, got %q", out[1].ContainerName)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
