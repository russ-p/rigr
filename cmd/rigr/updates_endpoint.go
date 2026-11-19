package main

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

// UpdateRelease is a single pending release with lossless fields: the exact
// release title, the match version derived from it, and a prerelease flag.
type UpdateRelease struct {
	Title           string         `json:"title"`
	MatchVersion    string         `json:"match_version,omitempty"`
	ReleaseNotesURL string         `json:"release_notes_url"`
	PublishedAt     *time.Time     `json:"published_at,omitempty"`
	Severity        UpdateSeverity `json:"severity,omitempty"`
	Prerelease      bool           `json:"prerelease"`
}

// UpdateItem is the machine-oriented view of one tracked app.
type UpdateItem struct {
	ContainerName       string          `json:"container_name"`
	ContainerID         string          `json:"container_id"`
	Image               string          `json:"image"`
	CurrentTag          string          `json:"current_tag"`
	CurrentMatchVersion string          `json:"current_match_version,omitempty"`
	ReleaseFeed         string          `json:"release_feed"`
	MatchStatus         string          `json:"match_status"`
	Severity            UpdateSeverity  `json:"severity,omitempty"`
	LastCheckedAt       *time.Time      `json:"last_checked_at,omitempty"`
	Recommended         *UpdateRelease  `json:"recommended"`
	UpdatesAvailable    []UpdateRelease `json:"updates_available"`
}

// UpdatesOptions controls BuildUpdates filtering and classification.
type UpdatesOptions struct {
	// Container keeps only the app with this exact container name.
	Container string
	// IncludeUpToDate adds apps with no pending updates.
	IncludeUpToDate bool
	// PrereleaseRe marks releases as pre-releases by exact release title.
	PrereleaseRe *regexp.Regexp
	// FeedVersionRe extracts match_version from release titles/links.
	FeedVersionRe *regexp.Regexp
}

// BuildUpdates returns a machine-oriented list of pending updates.
//
// Behavior:
//   - Includes only apps with confirmed updates_available (len > 0) unless
//     IncludeUpToDate is set; apps without updates still get an empty (non-nil)
//     updates_available array.
//   - `current_tag` is the exact running image tag (AppState.CurrentVersion),
//     with variant suffixes preserved.
//   - `recommended` is the newest non-pre-release entry in updates_available,
//     or null when only pre-releases are pending. Callers should not upgrade
//     automatically in that case.
//   - App-level `severity` is the max severity across updates_available, as in
//     BuildHomepageUpdates.
//   - Sorts by the newest pending release published_at first (missing
//     timestamps last), then by container_name.
func BuildUpdates(apps []AppState, opts UpdatesOptions) []UpdateItem {
	out := make([]UpdateItem, 0)

	for _, app := range apps {
		if opts.Container != "" && app.ContainerName != opts.Container {
			continue
		}
		if len(app.UpdatesAvailable) == 0 && !opts.IncludeUpToDate {
			continue
		}

		releases := make([]UpdateRelease, 0, len(app.UpdatesAvailable))
		for _, u := range app.UpdatesAvailable {
			releases = append(releases, buildUpdateRelease(u, opts))
		}

		sev := UpdateSeverityDefault
		for _, u := range app.UpdatesAvailable {
			sev = MaxSeverity(sev, u.Severity)
		}

		item := UpdateItem{
			ContainerName:       app.ContainerName,
			ContainerID:         app.ContainerID,
			Image:               app.Image,
			CurrentTag:          app.CurrentVersion,
			CurrentMatchVersion: app.CurrentMatchVersion,
			ReleaseFeed:         app.ReleaseFeed,
			MatchStatus:         app.MatchStatus,
			Severity:            sev,
			LastCheckedAt:       app.LastCheckedAt,
			Recommended:         pickRecommended(releases),
			UpdatesAvailable:    releases,
		}
		out = append(out, item)
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		at, bt := newestPendingPublishedAt(a), newestPendingPublishedAt(b)

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

func buildUpdateRelease(u AppUpdate, opts UpdatesOptions) UpdateRelease {
	title := strings.TrimSpace(u.Title)
	link := strings.TrimSpace(u.ReleaseNotesURL)
	return UpdateRelease{
		Title:           u.Title,
		MatchVersion:    releaseMatchVersion(opts.FeedVersionRe, title, link),
		ReleaseNotesURL: u.ReleaseNotesURL,
		PublishedAt:     u.PublishedAt,
		Severity:        u.Severity,
		Prerelease:      IsPrerelease(opts.PrereleaseRe, title, link),
	}
}

// IsPrerelease reports whether a release is a pre-release by its exact title,
// falling back to the release notes URL when the title is empty.
func IsPrerelease(re *regexp.Regexp, title, link string) bool {
	if re == nil {
		return false
	}
	name := strings.TrimSpace(title)
	if name == "" {
		name = strings.TrimSpace(link)
	}
	return name != "" && re.MatchString(name)
}

// pickRecommended returns the newest non-pre-release entry (feed order is
// newest-first, so ties keep the earlier entry), or nil when every pending
// update is a pre-release.
func pickRecommended(updates []UpdateRelease) *UpdateRelease {
	var best *UpdateRelease
	for i := range updates {
		u := &updates[i]
		if u.Prerelease {
			continue
		}
		if best == nil || newerRelease(u, best) {
			best = u
		}
	}
	return best
}

func newerRelease(a, b *UpdateRelease) bool {
	at, bt := a.PublishedAt, b.PublishedAt
	switch {
	case at == nil:
		return false
	case bt == nil:
		return true
	default:
		return at.After(*bt)
	}
}

func newestPendingPublishedAt(item UpdateItem) *time.Time {
	if len(item.UpdatesAvailable) == 0 {
		return nil
	}
	return item.UpdatesAvailable[0].PublishedAt
}

func releaseMatchVersion(re *regexp.Regexp, title, link string) string {
	if v := ExtractVersionMatch(re, title); v != "" {
		return v
	}
	return ExtractVersionMatch(re, link)
}

// parseBoolParam accepts the same truthy values as getenvBool, defaulting to
// false for anything else (including an absent value).
func parseBoolParam(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}
