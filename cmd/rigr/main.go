package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gorilla/feeds"
	"github.com/mmcdole/gofeed"
)

const (
	defaultLabelPrefix     = "rigr."
	labelReleaseFeed       = "release_feed"
	labelName              = "name"
	labelImageVersionRegex = "image_version_regex"
	labelFeedVersionRegex  = "feed_version_regex"
	labelSkipVersionRegex  = "skip_version_regex"
)

type Config struct {
	LabelPrefix            string
	PollInterval           time.Duration
	HTTPBind               string
	HTTPTimeout            time.Duration
	UserAgent              string
	MaxFeedEntries         int
	UpdateSeverityEnabled  bool
}

func readConfig() (Config, error) {
	cfg := Config{
		LabelPrefix:           getenv("LABEL_PREFIX", defaultLabelPrefix),
		PollInterval:          getenvDuration("POLL_INTERVAL", 15*time.Minute),
		HTTPBind:              getenv("HTTP_BIND", "0.0.0.0:8080"),
		HTTPTimeout:           getenvDuration("HTTP_TIMEOUT", 10*time.Second),
		UserAgent:             getenv("USER_AGENT", "rigr/0.1.0 (+https://github.com/)"),
		MaxFeedEntries:        getenvInt("MAX_FEED_ENTRIES", 50),
		UpdateSeverityEnabled: getenvBool("UPDATE_SEVERITY_ENABLED", false),
	}
	if cfg.MaxFeedEntries <= 0 {
		return Config{}, fmt.Errorf("MAX_FEED_ENTRIES must be > 0")
	}
	if cfg.PollInterval <= 0 {
		return Config{}, fmt.Errorf("POLL_INTERVAL must be > 0")
	}
	if strings.TrimSpace(cfg.LabelPrefix) == "" {
		return Config{}, fmt.Errorf("LABEL_PREFIX must be non-empty")
	}
	return cfg, nil
}

type AppUpdate struct {
	Title           string     `json:"title"`
	ReleaseNotesURL string     `json:"release_notes_url"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
	Severity        UpdateSeverity `json:"severity,omitempty"`
}

type AppState struct {
	ContainerID         string      `json:"container_id"`
	ContainerName       string      `json:"container_name"`
	Image               string      `json:"image"`
	CurrentVersion      string      `json:"current_version"`
	CurrentMatchVersion string      `json:"current_match_version,omitempty"`
	ReleaseFeed         string      `json:"release_feed"`
	MatchStatus         string      `json:"match_status"` // matched | no_match | no_version
	UpdatesAvailable    []AppUpdate `json:"updates_available"`
	LatestKnownRelease  *AppUpdate  `json:"latest_known_release,omitempty"`
	LatestMatchVersion  string      `json:"latest_match_version,omitempty"`
	LastCheckedAt       *time.Time  `json:"last_checked_at,omitempty"`
	LastFeedFetchOK     bool        `json:"last_feed_fetch_ok"`
	LastError           string      `json:"last_error,omitempty"`
}

type Store struct {
	mu   sync.RWMutex
	apps map[string]AppState
}

func NewStore() *Store {
	return &Store{
		apps: make(map[string]AppState),
	}
}

// ReplaceAll swaps in the snapshot from the latest successful Docker poll.
// Containers recreated after an image update get a new ID; replacing the map
// drops stale entries that would otherwise keep serving old state via the API.
func (s *Store) ReplaceAll(next map[string]AppState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apps = make(map[string]AppState, len(next))
	for id, app := range next {
		s.apps[id] = app
	}
}

func (s *Store) List() []AppState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AppState, 0, len(s.apps))
	for _, v := range s.apps {
		out = append(out, v)
	}
	return out
}

func (s *Store) Get(id string) (AppState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.apps[id]
	return v, ok
}

func main() {
	logLevelFlag := flag.String("log-level", "", "log level: debug|info|warn|error (overrides LOG_LEVEL)")
	flag.Parse()

	level := getenv("LOG_LEVEL", "info")
	if strings.TrimSpace(*logLevelFlag) != "" {
		level = *logLevelFlag
	}
	logger := newLogger(level)
	slog.SetDefault(logger)

	cfg, err := readConfig()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	docker, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		slog.Error("docker client error", "err", err)
		os.Exit(1)
	}

	store := NewStore()

	httpClient := &http.Client{
		Timeout: cfg.HTTPTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          50,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	poller := &Poller{
		Cfg:        cfg,
		Docker:     docker,
		HTTPClient: httpClient,
		FeedParser: gofeed.NewParser(),
		Store:      store,
		Logger:     logger,
	}

	go poller.Run(ctx)

	srv := &http.Server{
		Addr:              cfg.HTTPBind,
		ReadHeaderTimeout: 5 * time.Second,
		Handler:           routes(cfg, docker, store),
	}

	go func() {
		<-ctx.Done()
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("listening", "bind", cfg.HTTPBind)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("http server error", "err", err)
		os.Exit(1)
	}
}

type Poller struct {
	Cfg        Config
	Docker     *client.Client
	HTTPClient *http.Client
	FeedParser *gofeed.Parser
	Store      *Store
	Logger     *slog.Logger
}

func (p *Poller) Run(ctx context.Context) {
	t := time.NewTicker(p.Cfg.PollInterval)
	defer t.Stop()

	p.pollOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.pollOnce(ctx)
		}
	}
}

func (p *Poller) pollOnce(ctx context.Context) {
	containers, err := p.Docker.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		p.Logger.Error("docker container list error", "err", err)
		return
	}

	type pollSummary struct {
		tracked      int
		fetchOK      int
		fetchFailed  int
		needsUpdate  int
		noMatch      int
		noVersion    int
		matchedNoUpd int
	}
	var sum pollSummary
	next := make(map[string]AppState)

	for _, c := range containers {
		feedURL := strings.TrimSpace(c.Labels[p.Cfg.LabelPrefix+labelReleaseFeed])
		if feedURL == "" {
			continue
		}
		sum.tracked++

		appName := strings.TrimSpace(c.Labels[p.Cfg.LabelPrefix+labelName])
		if appName == "" {
			appName = firstContainerName(c.Names)
		}

		imageRef := c.Image
		_, tag := splitImageTag(imageRef)
		current := tag

		imageVerReRaw := strings.TrimSpace(c.Labels[p.Cfg.LabelPrefix+labelImageVersionRegex])
		feedVerReRaw := strings.TrimSpace(c.Labels[p.Cfg.LabelPrefix+labelFeedVersionRegex])
		skipVerReRaw := strings.TrimSpace(c.Labels[p.Cfg.LabelPrefix+labelSkipVersionRegex])
		extractors := CompileVersionExtractors(p.Logger, imageVerReRaw, feedVerReRaw)
		skipVersionRe := CompileSkipVersionRegex(p.Logger, skipVerReRaw)
		currentMatchVersion := ExtractVersion(extractors.Image, current)

		p.Logger.Debug("checking app",
			"app", appName,
			"container_id", c.ID,
			"image", imageRef,
			"current_version", current,
			"match_version", currentMatchVersion,
			"release_feed", feedURL,
		)

		state := AppState{
			ContainerID:         c.ID,
			ContainerName:       appName,
			Image:               imageRef,
			CurrentVersion:      current,
			CurrentMatchVersion: currentMatchVersion,
			ReleaseFeed:         feedURL,
			MatchStatus:         "no_version",
		}

		now := time.Now().UTC()
		state.LastCheckedAt = &now

		if current != "" {
			state.MatchStatus = "no_match"
		}

		updates, latest, latestMatchVersion, matchStatus, fetchOK, lastErr := p.checkFeed(ctx, feedURL, currentMatchVersion, extractors.Feed, skipVersionRe)
		state.UpdatesAvailable = updates
		state.LatestKnownRelease = latest
		state.LatestMatchVersion = latestMatchVersion
		if matchStatus != "" {
			state.MatchStatus = matchStatus
		}
		state.LastFeedFetchOK = fetchOK
		state.LastError = lastErr

		if fetchOK {
			sum.fetchOK++
		} else {
			sum.fetchFailed++
		}
		if len(updates) > 0 {
			sum.needsUpdate++
		} else {
			switch state.MatchStatus {
			case "no_match":
				sum.noMatch++
			case "no_version":
				sum.noVersion++
			case "matched":
				sum.matchedNoUpd++
			}
		}

		if fetchOK {
			p.Logger.Debug("checked app",
				"app", appName,
				"container_id", c.ID,
				"match_status", state.MatchStatus,
				"updates_available", len(updates),
			)
		} else {
			p.Logger.Debug("checked app (feed error)",
				"app", appName,
				"container_id", c.ID,
				"match_status", state.MatchStatus,
				"err", lastErr,
			)
		}

		next[state.ContainerID] = state
	}

	p.Store.ReplaceAll(next)

	p.Logger.Info("poll summary",
		"tracked", sum.tracked,
		"success", sum.fetchOK,
		"failed", sum.fetchFailed,
		"needs_update", sum.needsUpdate,
		"no_match", sum.noMatch,
		"no_version", sum.noVersion,
		"matched_no_updates", sum.matchedNoUpd,
	)
}

func (p *Poller) checkFeed(ctx context.Context, url string, currentMatchVersion string, feedVersionExtractor, skipVersionRe *regexp.Regexp) ([]AppUpdate, *AppUpdate, string, string, bool, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, "", "no_match", false, err.Error()
	}
	req.Header.Set("User-Agent", p.Cfg.UserAgent)

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, "", "no_match", false, err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, "", "no_match", false, fmt.Sprintf("feed http status: %s", resp.Status)
	}

	feed, err := p.FeedParser.Parse(resp.Body)
	if err != nil {
		return nil, nil, "", "no_match", false, err.Error()
	}

	items := feed.Items
	if len(items) == 0 {
		return nil, nil, "", "no_match", true, ""
	}
	if p.Cfg.MaxFeedEntries > 0 && len(items) > p.Cfg.MaxFeedEntries {
		items = items[:p.Cfg.MaxFeedEntries]
	}
	items = FilterSkippedFeedItems(items, skipVersionRe)
	if len(items) == 0 {
		return nil, nil, "", "no_match", true, ""
	}

	latest := itemToUpdate(items[0], p.Cfg.UpdateSeverityEnabled)
	latestMatchVersion := ExtractFeedVersion(feedVersionExtractor, items[0])

	if strings.TrimSpace(currentMatchVersion) == "" {
		return nil, latest, latestMatchVersion, "no_version", true, ""
	}

	matchIdx := FindMatchingFeedIndex(items, feedVersionExtractor, currentMatchVersion)

	if matchIdx == -1 {
		// Spec: mark as no_match, report latest_known_release, but do not claim updates.
		return nil, latest, latestMatchVersion, "no_match", true, ""
	}

	if matchIdx == 0 {
		return nil, latest, latestMatchVersion, "matched", true, ""
	}

	updates := make([]AppUpdate, 0, matchIdx)
	for _, it := range items[:matchIdx] {
		u := itemToUpdate(it, p.Cfg.UpdateSeverityEnabled)
		if u == nil {
			continue
		}
		updates = append(updates, *u)
	}
	return updates, latest, latestMatchVersion, "matched", true, ""
}

func routes(cfg Config, docker *client.Client, store *Store) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		type healthResp struct {
			OK        bool   `json:"ok"`
			DockerOK  bool   `json:"docker_ok"`
			Timestamp string `json:"timestamp"`
		}
		dockerOK := true
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		_, err := docker.Ping(ctx)
		if err != nil {
			dockerOK = false
		}
		out := healthResp{OK: true, DockerOK: dockerOK, Timestamp: time.Now().UTC().Format(time.RFC3339)}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("/api/v1/apps", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, store.List())
	})

	mux.HandleFunc("/api/v1/homepage/updates", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		out := BuildHomepageUpdates(store.List())
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("/api/v1/apps/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/apps/")
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		app, ok := store.Get(id)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, app)
	})

	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		xml, err := buildAggregatedRSS(store.List())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		_, _ = w.Write([]byte(xml))
	})

	mux.HandleFunc("/api/v1/updates.rss", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		xml, err := buildAggregatedRSS(store.List())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		_, _ = w.Write([]byte(xml))
	})

	return mux
}

func buildAggregatedRSS(apps []AppState) (string, error) {
	now := time.Now().UTC()
	feed := &feeds.Feed{
		Title:       "rigr updates",
		Link:        &feeds.Link{Href: "http://localhost/feed.xml"},
		Description: "Aggregated updates detected from container release feeds.",
		Author:      &feeds.Author{Name: "rigr"},
		Created:     now,
	}

	for _, app := range apps {
		if len(app.UpdatesAvailable) == 0 {
			continue
		}
		top := app.UpdatesAvailable[0]
		sev := UpdateSeverityDefault
		for _, u := range app.UpdatesAvailable {
			sev = MaxSeverity(sev, u.Severity)
		}
		prefix := SeverityEmoji(sev)

		desc := new(strings.Builder)
		fmt.Fprintf(desc, "Container: %s\nImage: %s\nCurrent: %s\n\nUpdates:\n", app.ContainerName, app.Image, app.CurrentVersion)
		for _, u := range app.UpdatesAvailable {
			if u.PublishedAt != nil {
				fmt.Fprintf(desc, "- %s (%s)\n  %s\n", u.Title, u.PublishedAt.UTC().Format(time.RFC3339), u.ReleaseNotesURL)
			} else {
				fmt.Fprintf(desc, "- %s\n  %s\n", u.Title, u.ReleaseNotesURL)
			}
		}

		title := fmt.Sprintf("%s: %d update(s) available", app.ContainerName, len(app.UpdatesAvailable))
		if prefix != "" {
			title = fmt.Sprintf("%s %s", prefix, title)
		}

		item := &feeds.Item{
			Title:       title,
			Link:        &feeds.Link{Href: top.ReleaseNotesURL},
			Description: desc.String(),
			Id:          app.ContainerID,
		}
		if top.PublishedAt != nil {
			item.Created = top.PublishedAt.UTC()
		} else {
			item.Created = now
		}
		feed.Items = append(feed.Items, item)
	}

	return feed.ToRss()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func firstContainerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	n := strings.TrimSpace(names[0])
	return strings.TrimPrefix(n, "/")
}

func splitImageTag(image string) (name string, tag string) {
	if image == "" {
		return "", ""
	}
	if at := strings.Index(image, "@"); at >= 0 {
		image = image[:at]
	}
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon > lastSlash {
		return image[:lastColon], image[lastColon+1:]
	}
	return image, "latest"
}

func itemToUpdate(it *gofeed.Item, severityEnabled bool) *AppUpdate {
	if it == nil {
		return nil
	}
	u := &AppUpdate{
		Title:           strings.TrimSpace(it.Title),
		ReleaseNotesURL: strings.TrimSpace(it.Link),
	}
	if u.Title == "" {
		u.Title = "release"
	}
	if severityEnabled {
		u.Severity = ClassifySeverity(feedItemText(it))
	}
	var t *time.Time
	if it.PublishedParsed != nil {
		tt := it.PublishedParsed.UTC()
		t = &tt
	} else if it.UpdatedParsed != nil {
		tt := it.UpdatedParsed.UTC()
		t = &tt
	}
	u.PublishedAt = t
	return u
}

func feedItemText(it *gofeed.Item) string {
	if it == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	if s := strings.TrimSpace(it.Title); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimSpace(it.Description); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimSpace(it.Content); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n")
}

func getenv(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func getenvBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "t", "yes", "y", "on":
		return true
	case "0", "false", "f", "no", "n", "off":
		return false
	default:
		return def
	}
}

func getenvInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getenvDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "info", "":
		lvl = slog.LevelInfo
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: lvl,
	})
	return slog.New(h)
}
