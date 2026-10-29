package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeGitHub answers /repos/<repo>/compare/<base>...main like the GitHub API.
func fakeGitHub(t *testing.T, ahead int) (*httptest.Server, *string) {
	t.Helper()
	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Path
		type cm struct {
			SHA    string `json:"sha"`
			Commit struct {
				Message   string `json:"message"`
				Committer struct {
					Date time.Time `json:"date"`
				} `json:"committer"`
			} `json:"commit"`
		}
		var commits []cm
		for i := 1; i <= ahead; i++ {
			var c cm
			c.SHA = strings.Repeat(string(rune('a'+i)), 40)
			c.Commit.Message = "change number " + itoa(i) + "\n\nbody"
			c.Commit.Committer.Date = time.Date(2026, 10, 20+i, 0, 0, 0, 0, time.UTC)
			commits = append(commits, c)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "behind", "ahead_by": ahead, "commits": commits})
	}))
	t.Cleanup(srv.Close)
	return srv, &asked
}

func checkerFor(srv *httptest.Server, current string) *UpdateChecker {
	u := NewUpdateChecker(UpdateConfig{Deployment: "easypanel"}, nil)
	u.APIBase, u.Current = srv.URL, func() string { return current }
	return u
}

func TestUpdateCheckFindsNewerVersionAndChanges(t *testing.T) {
	srv, asked := fakeGitHub(t, 3)
	u := checkerFor(srv, "0123456789abcdef0123456789abcdef01234567")
	u.Check(context.Background())
	s := u.Status()
	if *asked != "/repos/magnusfroste/sluss-eu/compare/0123456789abcdef0123456789abcdef01234567...main" {
		t.Fatalf("asked %q", *asked)
	}
	if !s.Available || s.Behind != 3 || len(s.Commits) != 3 || s.Commits[0].Subject != "change number 3" {
		t.Fatalf("status = %+v", s)
	}
	if s.Latest != strings.Repeat("d", 40) || s.Deploy != "easypanel" {
		t.Fatalf("latest/deploy wrong: %+v", s)
	}
}

func TestUpdateCheckUpToDateDevAndDisabled(t *testing.T) {
	srv, _ := fakeGitHub(t, 0)
	u := checkerFor(srv, "0123456789abcdef0123456789abcdef01234567")
	u.Check(context.Background())
	if s := u.Status(); s.Available || s.Error != "" || s.Checked.IsZero() {
		t.Fatalf("up to date: %+v", s)
	}
	dev := checkerFor(srv, "dev")
	dev.Check(context.Background())
	if s := dev.Status(); s.Available || !strings.Contains(s.Error, "development build") {
		t.Fatalf("dev build: %+v", s)
	}
	off := NewUpdateChecker(UpdateConfig{Disabled: true}, nil)
	if s := off.Status(); !s.Disabled || s.Available {
		t.Fatalf("disabled: %+v", s)
	}
	var none *UpdateChecker
	if s := none.Status(); !s.Disabled {
		t.Fatal("nil checker should read as disabled")
	}
}

func TestVersionPageFooterDotAndOverviewInfo(t *testing.T) {
	srv, _ := fakeGitHub(t, 2)
	u := checkerFor(srv, "0123456789abcdef0123456789abcdef01234567")
	u.Check(context.Background())
	activeUpdates = u
	t.Cleanup(func() { activeUpdates = nil })

	rec := httptest.NewRecorder()
	VersionPageHandler(u, nil)(rec, httptest.NewRequest(http.MethodGet, "/router/system", nil))
	body := rec.Body.String()
	for _, want := range []string{"2 changes behind", "change number 2", "EasyPanel", "docker compose pull", "Sluss never upgrades itself", "Check now"} {
		if !strings.Contains(body, want) {
			t.Fatalf("version page lacks %q", want)
		}
	}
	// EasyPanel is configured, so it is the first (open) path.
	if strings.Index(body, ">EasyPanel<") > strings.Index(body, ">Docker<") {
		t.Fatal("the configured deployment should come first")
	}
	if !strings.Contains(adminNavHTML("dashboard"), `class="tk-upd"`) {
		t.Fatal("menu footer should carry the update dot")
	}
	items := buildAttention(DashboardOptions{}, &DashboardData{Setup: SetupStatus{CloudModels: []string{"x"}, LocalModels: []string{"y"}, Protects: true, Requests: 1}}, time.Now())
	var info bool
	for _, it := range items {
		info = info || (it.Level == "info" && it.Href == "/router/system")
	}
	if !info {
		t.Fatalf("overview should mention the update: %+v", items)
	}
}
