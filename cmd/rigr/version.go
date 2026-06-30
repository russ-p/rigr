package main

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/mmcdole/gofeed"
)

// Defaults implement "strip image variant" semantics:
// - image tag `26.5.0-alpine` -> `26.5.0`
// - feed title `v26.5.2` -> `26.5.2`
const (
	defaultImageVersionRegex = `(?i)\bv?(\d+\.\d+\.\d+)\b`
	defaultFeedVersionRegex  = `(?i)\bv?(\d+\.\d+\.\d+)\b`
	// Matches pre-release suffixes like -dev, -rc, -rc1, 2026.7.0b2 on feed titles/links.
	defaultSkipVersionRegex = `(?i)(?:[-.]?(?:dev|rc\d*)|b\d+)$`
)

type VersionExtractors struct {
	Image *regexp.Regexp
	Feed  *regexp.Regexp
}

func CompileVersionExtractors(logger *slog.Logger, imageRegex, feedRegex string) VersionExtractors {
	img := compileExtractor(logger, "image_version_regex", imageRegex, defaultImageVersionRegex)
	fd := compileExtractor(logger, "feed_version_regex", feedRegex, defaultFeedVersionRegex)
	return VersionExtractors{Image: img, Feed: fd}
}

// CompileSkipVersionRegex returns a regex for feed items that should be ignored.
// Empty pattern uses defaultSkipVersionRegex. Set pattern to "-", "off", or "none" to disable.
func CompileSkipVersionRegex(logger *slog.Logger, pattern string) *regexp.Regexp {
	pattern = strings.TrimSpace(pattern)
	switch strings.ToLower(pattern) {
	case "", defaultSkipVersionRegex:
		pattern = defaultSkipVersionRegex
	case "-", "off", "none":
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		if logger != nil {
			logger.Warn("invalid skip version regex; using default",
				"label", "skip_version_regex",
				"pattern", pattern,
				"err", err,
			)
		}
		return regexp.MustCompile(defaultSkipVersionRegex)
	}
	return re
}

func compileExtractor(logger *slog.Logger, labelName, pattern, fallback string) *regexp.Regexp {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		pattern = fallback
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		if logger != nil {
			logger.Warn("invalid version regex; using default",
				"label", labelName,
				"pattern", pattern,
				"err", err,
			)
		}
		re = regexp.MustCompile(fallback)
	}
	return re
}

func NormalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "Release ")
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimSpace(v)
	return v
}

// ExtractVersion returns a normalized version extracted from s using re.
// Extraction rules:
// - If no match -> NormalizeVersion(s)
// - If match and there is capture group 1 -> NormalizeVersion(group1)
// - Else -> NormalizeVersion(full match)
func ExtractVersion(re *regexp.Regexp, s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if re == nil {
		return NormalizeVersion(s)
	}

	m := re.FindStringSubmatch(s)
	if len(m) == 0 {
		return NormalizeVersion(s)
	}
	if len(m) > 1 && strings.TrimSpace(m[1]) != "" {
		return NormalizeVersion(m[1])
	}
	return NormalizeVersion(m[0])
}

// ExtractVersionMatch is like ExtractVersion, but returns "" when re doesn't match.
// This is useful for feed parsing where we want to try multiple sources (Title, then Link)
// and not treat arbitrary titles as a "version".
func ExtractVersionMatch(re *regexp.Regexp, s string) string {
	s = strings.TrimSpace(s)
	if s == "" || re == nil {
		return ""
	}
	m := re.FindStringSubmatch(s)
	if len(m) == 0 {
		return ""
	}
	if len(m) > 1 && strings.TrimSpace(m[1]) != "" {
		return NormalizeVersion(m[1])
	}
	return NormalizeVersion(m[0])
}

func ShouldSkipFeedItem(skipRe *regexp.Regexp, it *gofeed.Item) bool {
	if skipRe == nil || it == nil {
		return false
	}
	if s := strings.TrimSpace(it.Title); s != "" && skipRe.MatchString(s) {
		return true
	}
	if s := strings.TrimSpace(it.Link); s != "" && skipRe.MatchString(s) {
		return true
	}
	return false
}

func FilterSkippedFeedItems(items []*gofeed.Item, skipRe *regexp.Regexp) []*gofeed.Item {
	if skipRe == nil || len(items) == 0 {
		return items
	}
	out := make([]*gofeed.Item, 0, len(items))
	for _, it := range items {
		if !ShouldSkipFeedItem(skipRe, it) {
			out = append(out, it)
		}
	}
	return out
}

func ExtractFeedVersion(feedRe *regexp.Regexp, it *gofeed.Item) string {
	if it == nil {
		return ""
	}
	if v := ExtractVersionMatch(feedRe, it.Title); v != "" {
		return v
	}
	return ExtractVersionMatch(feedRe, it.Link)
}

// FindMatchingFeedIndex returns the index in items where extracted feed version
// matches currentMatchVersion. Returns -1 when no match.
func FindMatchingFeedIndex(items []*gofeed.Item, feedRe *regexp.Regexp, currentMatchVersion string) int {
	cur := strings.TrimSpace(currentMatchVersion)
	if cur == "" {
		return -1
	}
	for i, it := range items {
		cand := ExtractFeedVersion(feedRe, it)
		if cand != "" && cand == cur {
			return i
		}
	}
	return -1
}
