package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type HomepageUpdateItem struct {
	ContainerName string     `json:"container_name"`
	VersionLine   string     `json:"version_line"`
	ChangelogURL  string     `json:"changelog_url,omitempty"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	Severity      UpdateSeverity `json:"severity,omitempty"`
}

// BuildHomepageUpdates returns a Homepage-friendly list of update items.
//
// Behavior:
//   - Includes only apps with confirmed updates_available (len > 0). This avoids
//     listing "no_match" cases where we have latest_known_release but cannot
//     safely claim an update.
//   - Uses latest_known_release when present to build `version_line` and URLs;
//     falls back to the newest entry in updates_available otherwise.
//   - Uses the normalized version values used in matching when available:
//     current_match_version and latest_match_version.
//   - Sorts by newest published_at first (missing timestamps last), then by name.
func BuildHomepageUpdates(apps []AppState) []HomepageUpdateItem {
	out := make([]HomepageUpdateItem, 0)

	for _, app := range apps {
		if len(app.UpdatesAvailable) == 0 {
			continue
		}

		latest := app.LatestKnownRelease
		if latest == nil {
			// Poller should usually set LatestKnownRelease when feed has items,
			// but keep the endpoint resilient.
			u := app.UpdatesAvailable[0]
			latest = &u
		}

		cur := strings.TrimSpace(app.CurrentMatchVersion)
		if cur == "" {
			cur = strings.TrimSpace(app.CurrentVersion)
		}
		if cur == "" {
			cur = "unknown"
		}

		newest := strings.TrimSpace(app.LatestMatchVersion)
		if newest == "" {
			newest = strings.TrimSpace(latest.Title)
		}
		if newest == "" {
			newest = "release"
		}

		sev := UpdateSeverityDefault
		for _, u := range app.UpdatesAvailable {
			sev = MaxSeverity(sev, u.Severity)
		}

		versionLine := fmt.Sprintf("%s \u2192 %s", cur, newest)
		if prefix := SeverityEmoji(sev); prefix != "" {
			versionLine = fmt.Sprintf("%s %s", prefix, versionLine)
		}
		item := HomepageUpdateItem{
			ContainerName: app.ContainerName,
			VersionLine:   versionLine,
			ChangelogURL:  strings.TrimSpace(latest.ReleaseNotesURL),
			PublishedAt:   latest.PublishedAt,
			Severity:      sev,
		}
		out = append(out, item)
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		at, bt := a.PublishedAt, b.PublishedAt

		switch {
		case at == nil && bt == nil:
			return strings.ToLower(a.ContainerName) < strings.ToLower(b.ContainerName)
		case at == nil:
			return false
		case bt == nil:
			return true
		default:
			if !at.Equal(*bt) {
				return at.After(*bt)
			}
			return strings.ToLower(a.ContainerName) < strings.ToLower(b.ContainerName)
		}
	})

	return out
}
