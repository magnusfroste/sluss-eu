package server

// "A newer version is available" (ISSUE-138), modelled on AgentHotel but
// without its self-upgrade: a security gateway does not get to drive Docker
// on its host. Sluss checks the public repository in the background, at most
// every few hours, and shows how far behind it is and what changed. The
// upgrade itself is the operator's step, and the Version page says exactly
// which step that is for Docker, Compose, EasyPanel or AgentHotel.
//
// The check sends no data about the instance: one unauthenticated GET to the
// GitHub API for the public repository. ROUTER_UPDATE_CHECK=off turns it off
// for air-gapped installs; the page then says so.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/magnusfroste/sluss/internal/buildinfo"
)

// UpdateConfig is read from the environment in main.
type UpdateConfig struct {
	Disabled   bool   // ROUTER_UPDATE_CHECK=off
	Deployment string // ROUTER_DEPLOYMENT: docker | compose | easypanel | agenthotel
}

// updateCommit is one change the running instance does not have yet.
type updateCommit struct {
	SHA, Subject string
	Date         time.Time
}

// UpdateStatus is what the console shows.
type UpdateStatus struct {
	Disabled  bool
	Checked   time.Time
	Error     string
	Current   string // full commit of this build, "" when unknown (dev build)
	Release   string // release tag this build was made from ("" for main builds)
	Track     string // "release" (compared with the latest release) or "main"
	LatestTag string // newest release tag, in release mode
	Latest    string
	Behind    int // commits on main this build does not have
	Commits   []updateCommit
	RepoURL   string
	ImageRef  string
	Deploy    string
	Available bool
}

// UpdateChecker refreshes UpdateStatus in the background.
type UpdateChecker struct {
	Cfg      UpdateConfig
	Repo     string // owner/name
	APIBase  string // https://api.github.com (tests point it at a fake)
	Client   *http.Client
	Interval time.Duration
	Logger   *slog.Logger
	Current  func() string // defaults to buildinfo.FullCommit
	Release  func() string // defaults to buildinfo.Release

	mu     sync.Mutex
	status UpdateStatus
}

// NewUpdateChecker builds the checker for this repository.
func NewUpdateChecker(cfg UpdateConfig, logger *slog.Logger) *UpdateChecker {
	return &UpdateChecker{
		Cfg: cfg, Repo: "magnusfroste/sluss-eu", APIBase: "https://api.github.com",
		Client: &http.Client{Timeout: 10 * time.Second}, Interval: 6 * time.Hour, Logger: logger,
	}
}

// Start runs the first check after a short delay, then every Interval, until
// ctx ends. Never on the request path.
func (u *UpdateChecker) Start(ctx context.Context) {
	if u == nil || u.Cfg.Disabled {
		return
	}
	go func() {
		t := time.NewTimer(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				u.Check(ctx)
				t.Reset(u.Interval)
			}
		}
	}()
}

// Status returns the latest known state.
func (u *UpdateChecker) Status() UpdateStatus {
	if u == nil {
		return UpdateStatus{Disabled: true}
	}
	u.mu.Lock()
	s := u.status
	u.mu.Unlock()
	s.Disabled = u.Cfg.Disabled
	s.Deploy = u.Cfg.Deployment
	s.RepoURL = "https://github.com/" + u.Repo
	s.ImageRef = "ghcr.io/" + u.Repo + ":latest"
	if s.Current == "" {
		s.Current = u.current()
	}
	s.Release = u.release()
	s.Track = "main"
	if s.Release != "" {
		s.Track = "release"
	}
	return s
}

func (u *UpdateChecker) current() string {
	c := buildinfo.FullCommit
	if u.Current != nil {
		c = u.Current
	}
	sha := strings.TrimSpace(c())
	if sha == "" || sha == "dev" || strings.HasSuffix(sha, "-dirty") {
		return ""
	}
	return sha
}

func (u *UpdateChecker) release() string {
	if u.Release != nil {
		return strings.TrimSpace(u.Release())
	}
	return strings.TrimSpace(buildinfo.Release())
}

// latestRelease returns the newest published release tag ("" when none).
func (u *UpdateChecker) latestRelease(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(u.APIBase, "/")+"/repos/"+u.Repo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := u.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	var rel struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil || resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github.com answered HTTP %d", resp.StatusCode)
	}
	return rel.Tag, nil
}

// Check asks GitHub how this build compares with what it should follow: the
// newest release for a release build, main for a main build.
func (u *UpdateChecker) Check(ctx context.Context) {
	st := UpdateStatus{Checked: time.Now(), Current: u.current()}
	head := "main"
	if rel := u.release(); rel != "" {
		latest, err := u.latestRelease(ctx)
		if err != nil {
			st.Error = "could not read the latest release: " + err.Error()
			u.mu.Lock()
			u.status = st
			u.mu.Unlock()
			return
		}
		st.LatestTag = latest
		if latest == "" || latest == rel {
			u.mu.Lock()
			u.status = st
			u.mu.Unlock()
			return
		}
		head = latest
	}
	defer func() {
		u.mu.Lock()
		u.status = st
		u.mu.Unlock()
	}()
	if st.Current == "" {
		st.Error = "this build has no commit stamp (a development build), so it cannot be compared"
		return
	}
	var cmp struct {
		Status   string `json:"status"`
		AheadBy  int    `json:"ahead_by"`
		BehindBy int    `json:"behind_by"`
		Commits  []struct {
			SHA    string `json:"sha"`
			Commit struct {
				Message   string `json:"message"`
				Committer struct {
					Date time.Time `json:"date"`
				} `json:"committer"`
			} `json:"commit"`
		} `json:"commits"`
		Message string `json:"message"`
	}
	url := strings.TrimRight(u.APIBase, "/") + "/repos/" + u.Repo + "/compare/" + st.Current + "..." + head
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		st.Error = err.Error()
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := u.Client.Do(req)
	if err != nil {
		st.Error = "could not reach github.com: " + err.Error()
		return
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&cmp); err != nil {
		st.Error = "unexpected answer from github.com"
		return
	}
	if resp.StatusCode != http.StatusOK {
		st.Error = "github.com answered HTTP " + itoa(resp.StatusCode)
		if cmp.Message != "" {
			st.Error += ": " + cmp.Message
		}
		return
	}
	// compare base...head: ahead_by = commits on main this build lacks.
	st.Behind = cmp.AheadBy
	st.Available = st.Behind > 0
	for i := len(cmp.Commits) - 1; i >= 0 && len(st.Commits) < 15; i-- {
		c := cmp.Commits[i]
		subj, _, _ := strings.Cut(c.Commit.Message, "\n")
		st.Commits = append(st.Commits, updateCommit{SHA: c.SHA, Subject: subj, Date: c.Commit.Committer.Date})
	}
	if len(cmp.Commits) > 0 {
		st.Latest = cmp.Commits[len(cmp.Commits)-1].SHA
	} else {
		st.Latest = st.Current
	}
	if u.Logger != nil && st.Available {
		u.Logger.Info("a newer Sluss is available", "behind", st.Behind, "latest", shortCommit(st.Latest))
	}
}

func shortCommit(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}
