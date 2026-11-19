package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

func testUpdatesOptions() UpdatesOptions {
	return UpdatesOptions{
		PrereleaseRe:  CompilePrereleaseRegex(nil, ""),
		FeedVersionRe: regexp.MustCompile(defaultFeedVersionRegex),
	}
}

// specApps mirrors the live data from the /api/v1/updates spec.
func specApps() []AppState {
	giteaPublished := ptrTime(time.Date(2026, 11, 15, 19, 39, 47, 0, time.UTC))
	fbNewPublished := ptrTime(time.Date(2026, 11, 14, 0, 44, 1, 0, time.UTC))
	fbOldPublished := ptrTime(time.Date(2026, 10, 31, 13, 37, 41, 0, time.UTC))
	reposiliteNextPublished := ptrTime(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC))
	reposilitePublished := ptrTime(time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC))
	checked := ptrTime(time.Date(2026, 11, 19, 18, 30, 0, 0, time.UTC))

	return []AppState{
		{
			ContainerName:       "gitea",
			ContainerID:         "abc123",
			Image:               "docker.gitea.com/gitea:28.3.0",
			CurrentVersion:      "28.3.0",
			CurrentMatchVersion: "28.3.0",
			ReleaseFeed:         "https://github.com/go-gitea/gitea/releases.atom",
			MatchStatus:         "matched",
			LastCheckedAt:       checked,
			UpdatesAvailable: []AppUpdate{
				{Title: "v28.4.0", ReleaseNotesURL: "https://github.com/go-gitea/gitea/releases/tag/v28.4.0", PublishedAt: giteaPublished, Severity: UpdateSeveritySecurityFixes},
			},
			LatestKnownRelease: &AppUpdate{Title: "v28.4.0", ReleaseNotesURL: "https://github.com/go-gitea/gitea/releases/tag/v28.4.0", PublishedAt: giteaPublished},
			LatestMatchVersion: "28.4.0",
		},
		{
			ContainerName:       "filebrowser-quantum",
			ContainerID:         "def456",
			Image:               "ghcr.io/gtsteffaniak/filebrowser:2.0.2-stable-slim",
			CurrentVersion:      "2.0.2-stable-slim",
			CurrentMatchVersion: "2.0.2",
			ReleaseFeed:         "https://github.com/gtsteffaniak/filebrowser/releases.atom",
			MatchStatus:         "matched",
			LastCheckedAt:       checked,
			UpdatesAvailable: []AppUpdate{
				{Title: "v2.1.5-beta", ReleaseNotesURL: "https://github.com/gtsteffaniak/filebrowser/releases/tag/v2.1.5-beta", PublishedAt: fbNewPublished, Severity: UpdateSeveritySecurityFixes},
				{Title: "v2.1.4-beta", ReleaseNotesURL: "https://github.com/gtsteffaniak/filebrowser/releases/tag/v2.1.4-beta", PublishedAt: fbOldPublished, Severity: UpdateSeverityDefault},
			},
			LatestKnownRelease: &AppUpdate{Title: "v2.1.5-beta", ReleaseNotesURL: "https://github.com/gtsteffaniak/filebrowser/releases/tag/v2.1.5-beta", PublishedAt: fbNewPublished},
			LatestMatchVersion: "2.1.5",
		},
		{
			ContainerName:       "reposilite",
			ContainerID:         "ghi789",
			Image:               "ghcr.io/dzikoysk/reposilite:3.6.6",
			CurrentVersion:      "3.6.6",
			CurrentMatchVersion: "3.6.6",
			ReleaseFeed:         "https://github.com/dzikoysk/reposilite/releases.atom",
			MatchStatus:         "matched",
			LastCheckedAt:       checked,
			UpdatesAvailable: []AppUpdate{
				{Title: "3.6.7-next", ReleaseNotesURL: "https://github.com/dzikoysk/reposilite/releases/tag/3.6.7-next", PublishedAt: reposiliteNextPublished, Severity: UpdateSeverityDefault},
				{Title: "3.6.7", ReleaseNotesURL: "https://github.com/dzikoysk/reposilite/releases/tag/3.6.7", PublishedAt: reposilitePublished, Severity: UpdateSeverityDefault},
			},
			LatestKnownRelease: &AppUpdate{Title: "3.6.7-next", ReleaseNotesURL: "https://github.com/dzikoysk/reposilite/releases/tag/3.6.7-next", PublishedAt: reposiliteNextPublished},
			LatestMatchVersion: "3.6.7",
		},
	}
}

func TestBuildUpdates_SpecExample(t *testing.T) {
	out := BuildUpdates(specApps(), testUpdatesOptions())
	if len(out) != 3 {
		t.Fatalf("expected 3 items, got %d", len(out))
	}

	// Sorted by newest pending published_at first: gitea, filebrowser-quantum,
	// reposilite.
	if out[0].ContainerName != "gitea" {
		t.Fatalf("expected first item gitea, got %q", out[0].ContainerName)
	}
	if out[1].ContainerName != "filebrowser-quantum" {
		t.Fatalf("expected second item filebrowser-quantum, got %q", out[1].ContainerName)
	}
	if out[2].ContainerName != "reposilite" {
		t.Fatalf("expected third item reposilite, got %q", out[2].ContainerName)
	}

	gitea := out[0]
	if gitea.CurrentTag != "28.3.0" {
		t.Fatalf("unexpected current_tag: %q", gitea.CurrentTag)
	}
	if gitea.MatchStatus != "matched" {
		t.Fatalf("unexpected match_status: %q", gitea.MatchStatus)
	}
	if gitea.Severity != UpdateSeveritySecurityFixes {
		t.Fatalf("unexpected severity: %q", gitea.Severity)
	}
	if gitea.Recommended == nil {
		t.Fatalf("expected non-nil recommended for gitea")
	}
	if gitea.Recommended.Title != "v28.4.0" {
		t.Fatalf("unexpected recommended title: %q", gitea.Recommended.Title)
	}
	if gitea.Recommended.MatchVersion != "28.4.0" {
		t.Fatalf("unexpected recommended match_version: %q", gitea.Recommended.MatchVersion)
	}
	if gitea.Recommended.Prerelease {
		t.Fatalf("expected recommended prerelease=false for gitea")
	}
	if gitea.Recommended.Severity != UpdateSeveritySecurityFixes {
		t.Fatalf("unexpected recommended severity: %q", gitea.Recommended.Severity)
	}
	if len(gitea.UpdatesAvailable) != 1 {
		t.Fatalf("expected 1 pending update for gitea, got %d", len(gitea.UpdatesAvailable))
	}
	if gitea.UpdatesAvailable[0].Prerelease {
		t.Fatalf("expected prerelease=false for v28.4.0")
	}
}

func TestBuildUpdates_AllPrereleasesNoRecommended(t *testing.T) {
	out := BuildUpdates(specApps(), testUpdatesOptions())

	var fb *UpdateItem
	for i := range out {
		if out[i].ContainerName == "filebrowser-quantum" {
			fb = &out[i]
		}
	}
	if fb == nil {
		t.Fatalf("filebrowser-quantum item missing")
	}

	if fb.CurrentTag != "2.0.2-stable-slim" {
		t.Fatalf("expected exact current_tag with suffix, got %q", fb.CurrentTag)
	}
	if fb.CurrentMatchVersion != "2.0.2" {
		t.Fatalf("unexpected current_match_version: %q", fb.CurrentMatchVersion)
	}
	if fb.Recommended != nil {
		t.Fatalf("expected recommended=null when only pre-releases are pending, got %q", fb.Recommended.Title)
	}
	if len(fb.UpdatesAvailable) != 2 {
		t.Fatalf("expected 2 pending updates, got %d", len(fb.UpdatesAvailable))
	}
	for _, u := range fb.UpdatesAvailable {
		if !u.Prerelease {
			t.Fatalf("expected prerelease=true for %q", u.Title)
		}
	}
	if fb.UpdatesAvailable[0].Title != "v2.1.5-beta" || fb.UpdatesAvailable[0].MatchVersion != "2.1.5" {
		t.Fatalf("unexpected newest pending update: %+v", fb.UpdatesAvailable[0])
	}
	if fb.UpdatesAvailable[1].Title != "v2.1.4-beta" || fb.UpdatesAvailable[1].MatchVersion != "2.1.4" {
		t.Fatalf("unexpected oldest pending update: %+v", fb.UpdatesAvailable[1])
	}
}

func TestBuildUpdates_RecommendsStableOverNextVariant(t *testing.T) {
	out := BuildUpdates(specApps(), testUpdatesOptions())

	var reposilite *UpdateItem
	for i := range out {
		if out[i].ContainerName == "reposilite" {
			reposilite = &out[i]
		}
	}
	if reposilite == nil {
		t.Fatalf("reposilite item missing")
	}

	if reposilite.Recommended == nil {
		t.Fatalf("expected non-nil recommended for reposilite")
	}
	if reposilite.Recommended.Title != "3.6.7" {
		t.Fatalf("expected recommended 3.6.7, got %q", reposilite.Recommended.Title)
	}
	if reposilite.Recommended.Prerelease {
		t.Fatalf("expected recommended prerelease=false")
	}
	if !reposilite.UpdatesAvailable[0].Prerelease {
		t.Fatalf("expected prerelease=true for 3.6.7-next")
	}
}

func TestIsPrerelease_DefaultRegex(t *testing.T) {
	re := CompilePrereleaseRegex(nil, "")
	cases := []struct {
		title string
		want  bool
	}{
		{title: "v2.1.5-beta", want: true},
		{title: "3.6.7-next", want: true},
		{title: "2026.7.0b2", want: true},
		{title: "1.0-rc1", want: true},
		{title: "1.0.0-alpha.1", want: true},
		{title: "3.6.7", want: false},
		{title: "v28.4.0", want: false},
		{title: "2026.11.2", want: false},
		{title: "7.1.3-php8.3-apache", want: false},
		{title: "26.10.0-alpine", want: false},
		{title: "2.0.2-stable-slim", want: false},
	}
	for _, tc := range cases {
		if got := IsPrerelease(re, tc.title, ""); got != tc.want {
			t.Errorf("IsPrerelease(%q) = %v, want %v", tc.title, got, tc.want)
		}
	}
}

func TestIsPrerelease_FallsBackToLink(t *testing.T) {
	re := CompilePrereleaseRegex(nil, "")
	link := "https://example.com/releases/tag/v1.0.0-rc1"
	if !IsPrerelease(re, "", link) {
		t.Fatalf("expected prerelease=true via link fallback")
	}
	if IsPrerelease(re, "1.0.0", link) {
		t.Fatalf("expected title to take precedence over link")
	}
}

func TestBuildUpdates_ContainerFilter(t *testing.T) {
	apps := specApps()

	opts := testUpdatesOptions()
	opts.Container = "gitea"
	out := BuildUpdates(apps, opts)
	if len(out) != 1 {
		t.Fatalf("expected 1 item for container=gitea, got %d", len(out))
	}
	if out[0].ContainerName != "gitea" {
		t.Fatalf("expected gitea, got %q", out[0].ContainerName)
	}

	opts = testUpdatesOptions()
	opts.Container = "no-such-container"
	out = BuildUpdates(apps, opts)
	if len(out) != 0 {
		t.Fatalf("expected 0 items for unknown container, got %d", len(out))
	}
	if out == nil {
		t.Fatalf("expected empty (non-nil) slice for unknown container, so it encodes as []")
	}
}

func TestBuildUpdates_IncludeUpToDate(t *testing.T) {
	apps := append(specApps(), AppState{
		ContainerName:       "fresh",
		ContainerID:         "jkl012",
		Image:               "ghcr.io/org/fresh:1.0.0",
		CurrentVersion:      "1.0.0",
		CurrentMatchVersion: "1.0.0",
		ReleaseFeed:         "https://github.com/org/fresh/releases.atom",
		MatchStatus:         "matched",
		UpdatesAvailable:    nil,
	})

	out := BuildUpdates(apps, testUpdatesOptions())
	if len(out) != 3 {
		t.Fatalf("expected up-to-date apps excluded by default, got %d items", len(out))
	}

	opts := testUpdatesOptions()
	opts.IncludeUpToDate = true
	out = BuildUpdates(apps, opts)
	if len(out) != 4 {
		t.Fatalf("expected 4 items with include_up_to_date, got %d", len(out))
	}

	var fresh *UpdateItem
	for i := range out {
		if out[i].ContainerName == "fresh" {
			fresh = &out[i]
		}
	}
	if fresh == nil {
		t.Fatalf("fresh item missing with include_up_to_date")
	}
	if fresh.UpdatesAvailable == nil || len(fresh.UpdatesAvailable) != 0 {
		t.Fatalf("expected empty non-nil updates_available for up-to-date app")
	}
	if fresh.Recommended != nil {
		t.Fatalf("expected recommended=null for up-to-date app")
	}
}

func TestBuildUpdates_PreservesCurrentTagWithSuffix(t *testing.T) {
	apps := []AppState{
		{
			ContainerName:       "filebrowser-quantum",
			ContainerID:         "def456",
			Image:               "ghcr.io/gtsteffaniak/filebrowser:2.0.2-stable-slim",
			CurrentVersion:      "2.0.2-stable-slim",
			CurrentMatchVersion: "2.0.2",
			MatchStatus:         "matched",
			UpdatesAvailable: []AppUpdate{
				{Title: "v2.1.5-beta", ReleaseNotesURL: "https://github.com/gtsteffaniak/filebrowser/releases/tag/v2.1.5-beta"},
			},
		},
	}

	out := BuildUpdates(apps, testUpdatesOptions())
	if len(out) != 1 {
		t.Fatalf("expected 1 item, got %d", len(out))
	}
	if out[0].CurrentTag != "2.0.2-stable-slim" {
		t.Fatalf("expected current_tag as-is with suffix, got %q", out[0].CurrentTag)
	}
}

func TestBuildUpdates_JSONContract(t *testing.T) {
	out := BuildUpdates(specApps(), testUpdatesOptions())
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	s := string(b)

	if !strings.Contains(s, `"recommended":{"title":"v28.4.0"`) {
		t.Fatalf("expected recommended object for gitea, got %s", s)
	}
	if !strings.Contains(s, `"recommended":null`) {
		t.Fatalf("expected explicit recommended:null for all-prerelease app, got %s", s)
	}
	if !strings.Contains(s, `"prerelease":false`) {
		t.Fatalf("expected explicit prerelease:false for stable release, got %s", s)
	}
	if !strings.Contains(s, `"prerelease":true`) {
		t.Fatalf("expected prerelease:true for beta release, got %s", s)
	}

	apps := append(specApps(), AppState{
		ContainerName:       "fresh",
		ContainerID:         "jkl012",
		Image:               "ghcr.io/org/fresh:1.0.0",
		CurrentVersion:      "1.0.0",
		CurrentMatchVersion: "1.0.0",
		MatchStatus:         "matched",
		UpdatesAvailable:    nil,
	})
	opts := testUpdatesOptions()
	opts.IncludeUpToDate = true
	out = BuildUpdates(apps, opts)
	b, err = json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if s := string(b); !strings.Contains(s, `"updates_available":[]`) {
		t.Fatalf("expected empty array (not null) for up-to-date app, got %s", s)
	}
}

func TestUpdatesRoute_QueryParams(t *testing.T) {
	store := NewStore()
	appsMap := make(map[string]AppState)
	for _, a := range specApps() {
		appsMap[a.ContainerID] = a
	}
	appsMap["jkl012"] = AppState{
		ContainerName:       "fresh",
		ContainerID:         "jkl012",
		Image:               "ghcr.io/org/fresh:1.0.0",
		CurrentVersion:      "1.0.0",
		CurrentMatchVersion: "1.0.0",
		ReleaseFeed:         "https://github.com/org/fresh/releases.atom",
		MatchStatus:         "matched",
		UpdatesAvailable:    nil,
	}
	store.ReplaceAll(appsMap)

	handler := routes(Config{PrereleaseRegex: defaultPrereleaseRegex}, nil, store)

	get := func(url string) []UpdateItem {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", url, rec.Code)
		}
		var out []UpdateItem
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s: invalid JSON: %v", url, err)
		}
		return out
	}

	if out := get("/api/v1/updates"); len(out) != 3 {
		t.Fatalf("expected 3 items by default, got %d", len(out))
	}
	if out := get("/api/v1/updates?container=gitea"); len(out) != 1 || out[0].ContainerName != "gitea" {
		t.Fatalf("expected only gitea for container filter, got %+v", out)
	}
	if out := get("/api/v1/updates?container=nope"); len(out) != 0 {
		t.Fatalf("expected empty array for unknown container, got %d", len(out))
	}
	if out := get("/api/v1/updates?include_up_to_date=true"); len(out) != 4 {
		t.Fatalf("expected 4 items with include_up_to_date=true, got %d", len(out))
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/updates", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST, got %d", rec.Code)
	}
}
