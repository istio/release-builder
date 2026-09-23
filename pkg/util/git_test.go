// Copyright Istio Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-github/v35/github"

	"istio.io/release-builder/pkg/model"
)

// callTracker records which GitHub API endpoints were reached.
type callTracker struct {
	searchCalled   bool
	createPRCalled bool
	updatePRCalled bool
	labelsCalled   bool
	labelsSent     []string
}

type mockServerOpts struct {
	existingPRNumber               int  // 0 = no existing open PR
	searchFails                    bool // search API returns 500
	searchTotalCountWithEmptyItems bool // total_count > 0 but items array is empty
	prCreateFails                  bool // POST /pulls returns 422
	labelsFail                     bool // POST /issues/.../labels returns 500
}

// newMockGitHubServer creates an httptest.Server that simulates the subset of the
// GitHub API used by CreatePR, CreateOrUpdatePR, and openNewPR.
func newMockGitHubServer(t *testing.T, opts mockServerOpts) (*httptest.Server, *callTracker) {
	t.Helper()
	tracker := &callTracker{}
	mux := http.NewServeMux()

	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"login": "test-bot",
			"name":  "Test Bot",
			"email": "bot@test.com",
		})
	})

	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		tracker.searchCalled = true
		if opts.searchFails {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if opts.searchTotalCountWithEmptyItems {
			// total_count > 0 but no items — guards the len(results.Issues) > 0 check
			json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 1,
				"items":       []interface{}{},
			})
			return
		}
		if opts.existingPRNumber != 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 1,
				"items":       []map[string]interface{}{{"number": opts.existingPRNumber}},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 0,
			"items":       []interface{}{},
		})
	})

	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/pulls") && r.Method == http.MethodPost:
			tracker.createPRCalled = true
			if opts.prCreateFails {
				w.WriteHeader(http.StatusUnprocessableEntity)
				json.NewEncoder(w).Encode(map[string]string{"message": "Validation Failed"})
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   99,
				"html_url": "https://github.com/test-org/istio/pull/99",
			})
		case strings.Contains(path, "/pulls/") && r.Method == http.MethodPatch:
			tracker.updatePRCalled = true
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"number": opts.existingPRNumber})
		case strings.Contains(path, "/issues/") && strings.HasSuffix(path, "/labels") && r.Method == http.MethodPost:
			tracker.labelsCalled = true
			if opts.labelsFail {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// go-github sends labels as a plain JSON array of strings
			var labels []string
			json.NewDecoder(r.Body).Decode(&labels)
			tracker.labelsSent = labels
			json.NewEncoder(w).Encode([]map[string]string{{"name": "auto-merge"}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, tracker
}

// mockGitHubClientFor returns a github.Client whose BaseURL points at the test server.
func mockGitHubClientFor(t *testing.T, server *httptest.Server) (*github.Client, *github.User) {
	t.Helper()
	serverURL, err := url.Parse(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	client := github.NewClient(nil)
	client.BaseURL = serverURL
	user := &github.User{
		Login: github.String("test-bot"),
		Name:  github.String("Test Bot"),
		Email: github.String("bot@test.com"),
	}
	return client, user
}

// injectMocks overrides setupGithubClientFn and pushCommitFn for the duration of the test.
// pushChanges controls whether the stubbed PushCommit reports that there are changes to commit.
func injectMocks(t *testing.T, server *httptest.Server, pushChanges bool) {
	t.Helper()
	client, user := mockGitHubClientFor(t, server)

	origSetup := setupGithubClientFn
	origPush := pushCommitFn
	t.Cleanup(func() {
		setupGithubClientFn = origSetup
		pushCommitFn = origPush
	})

	setupGithubClientFn = func(token string) (*github.Client, context.Context, *github.User, error) {
		return client, context.Background(), user, nil
	}
	pushCommitFn = func(manifest model.Manifest, repo, branch, commitString string, dryrun bool, githubToken string, user github.User, force bool) (bool, error) {
		return pushChanges, nil
	}
}

// =============================================================================
// parseOrgRepo
// =============================================================================

func TestParseOrgRepo(t *testing.T) {
	cases := []struct {
		name     string
		gitURL   string
		wantOrg  string
		wantRepo string
	}{
		{"istio/istio", "https://github.com/istio/istio", "istio", "istio"},
		{"fork", "https://github.com/frherrer/istio", "frherrer", "istio"},
		{"release-builder", "https://github.com/istio/release-builder", "istio", "release-builder"},
		{"different org", "https://github.com/openshift/service-mesh", "openshift", "service-mesh"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			org, repo := parseOrgRepo(tc.gitURL)
			if org != tc.wantOrg {
				t.Errorf("org: got %q, want %q", org, tc.wantOrg)
			}
			if repo != tc.wantRepo {
				t.Errorf("repo: got %q, want %q", repo, tc.wantRepo)
			}
		})
	}
}

// =============================================================================
// openNewPR
// =============================================================================

func TestOpenNewPR_CreatesWithCorrectParams(t *testing.T) {
	server, tracker := newMockGitHubServer(t, mockServerOpts{})
	client, _ := mockGitHubClientFor(t, server)

	err := openNewPR(context.Background(), client, "istio", "istio",
		"feature-branch", "master",
		"Update BASE_VERSION to master-2024-01-01", "scan body",
		[]string{"auto-merge"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tracker.createPRCalled {
		t.Error("expected POST /pulls to be called")
	}
	if !tracker.labelsCalled {
		t.Error("expected labels to be applied")
	}
}

func TestOpenNewPR_AutoLabelForIstioNonEnvoy(t *testing.T) {
	server, tracker := newMockGitHubServer(t, mockServerOpts{})
	client, _ := mockGitHubClientFor(t, server)

	err := openNewPR(context.Background(), client, "istio", "istio",
		"br", "master", "title", "body", []string{"auto-merge"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, l := range tracker.labelsSent {
		if l == "release-notes-none" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected release-notes-none label for istio/istio, got %v", tracker.labelsSent)
	}
}

func TestOpenNewPR_NoAutoLabelForEnvoy(t *testing.T) {
	server, tracker := newMockGitHubServer(t, mockServerOpts{})
	client, _ := mockGitHubClientFor(t, server)

	err := openNewPR(context.Background(), client, "istio", "envoy",
		"br", "master", "title", "body", []string{"auto-merge"})
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range tracker.labelsSent {
		if l == "release-notes-none" {
			t.Errorf("release-notes-none must not be added for istio/envoy, got %v", tracker.labelsSent)
		}
	}
}

func TestOpenNewPR_NoAutoLabelForNonIstioOrg(t *testing.T) {
	server, tracker := newMockGitHubServer(t, mockServerOpts{})
	client, _ := mockGitHubClientFor(t, server)

	err := openNewPR(context.Background(), client, "frherrer", "istio",
		"br", "master", "title", "body", []string{"auto-merge"})
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range tracker.labelsSent {
		if l == "release-notes-none" {
			t.Errorf("release-notes-none must not be added for non-istio org, got %v", tracker.labelsSent)
		}
	}
}

func TestOpenNewPR_ErrorOnCreateFailure(t *testing.T) {
	server, _ := newMockGitHubServer(t, mockServerOpts{prCreateFails: true})
	client, _ := mockGitHubClientFor(t, server)

	err := openNewPR(context.Background(), client, "istio", "istio",
		"br", "master", "title", "body", nil)
	if err == nil {
		t.Error("expected error when PR creation fails")
	}
}

func TestOpenNewPR_ErrorOnLabelFailure(t *testing.T) {
	server, _ := newMockGitHubServer(t, mockServerOpts{labelsFail: true})
	client, _ := mockGitHubClientFor(t, server)

	err := openNewPR(context.Background(), client, "istio", "istio",
		"br", "master", "title", "body", []string{"auto-merge"})
	if err == nil {
		t.Error("expected error when label application fails")
	}
}

// =============================================================================
// CreateOrUpdatePR
// =============================================================================

func TestCreateOrUpdatePR_NoPRExists(t *testing.T) {
	server, tracker := newMockGitHubServer(t, mockServerOpts{})
	injectMocks(t, server, true)

	err := CreateOrUpdatePR(
		model.Manifest{Version: "master"}, "istio", "update-base-version-master",
		"Update BASE_VERSION to master-2024-01-01", "body",
		false, "token",
		"https://github.com/test-org/istio", "master",
		[]string{"auto-merge"}, "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tracker.searchCalled {
		t.Error("expected search API to be called")
	}
	if !tracker.createPRCalled {
		t.Error("expected POST /pulls to create new PR")
	}
	if tracker.updatePRCalled {
		t.Error("expected no PATCH /pulls when no existing PR")
	}
}

func TestCreateOrUpdatePR_ExistingPRUpdated(t *testing.T) {
	server, tracker := newMockGitHubServer(t, mockServerOpts{existingPRNumber: 42})
	injectMocks(t, server, true)

	err := CreateOrUpdatePR(
		model.Manifest{Version: "master"}, "istio", "update-base-version-master",
		"Update BASE_VERSION to master-2024-01-02", "body",
		false, "token",
		"https://github.com/test-org/istio", "master",
		[]string{"auto-merge"}, "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tracker.updatePRCalled {
		t.Error("expected PATCH /pulls/42 to update existing PR")
	}
	if tracker.createPRCalled {
		t.Error("expected no POST /pulls when an existing PR is found")
	}
}

func TestCreateOrUpdatePR_SearchTotalCountWithEmptyItems(t *testing.T) {
	// total_count > 0 but items is empty: must not panic, must fall through to create.
	// This guards the len(results.Issues) > 0 fix (vs. GetTotal() > 0).
	server, tracker := newMockGitHubServer(t, mockServerOpts{searchTotalCountWithEmptyItems: true})
	injectMocks(t, server, true)

	err := CreateOrUpdatePR(
		model.Manifest{Version: "master"}, "istio", "update-base-version-master",
		"Update BASE_VERSION to master-2024-01-01", "body",
		false, "token",
		"https://github.com/test-org/istio", "master",
		[]string{"auto-merge"}, "",
	)
	if err != nil {
		t.Fatalf("must not error or panic: %v", err)
	}
	if !tracker.createPRCalled {
		t.Error("expected new PR to be created when search items slice is empty despite total_count>0")
	}
	if tracker.updatePRCalled {
		t.Error("expected no update when search items slice is empty")
	}
}

func TestCreateOrUpdatePR_SearchAPIFails(t *testing.T) {
	// Search failure must warn and fall through to create a new PR (degraded mode).
	server, tracker := newMockGitHubServer(t, mockServerOpts{searchFails: true})
	injectMocks(t, server, true)

	err := CreateOrUpdatePR(
		model.Manifest{Version: "master"}, "istio", "update-base-version-master",
		"Update BASE_VERSION to master-2024-01-01", "body",
		false, "token",
		"https://github.com/test-org/istio", "master",
		[]string{"auto-merge"}, "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tracker.createPRCalled {
		t.Error("expected new PR to be created when search API fails")
	}
}

func TestCreateOrUpdatePR_NoChanges(t *testing.T) {
	// When PushCommit reports no changes, no GitHub calls should happen.
	server, tracker := newMockGitHubServer(t, mockServerOpts{})
	injectMocks(t, server, false)

	err := CreateOrUpdatePR(
		model.Manifest{Version: "master"}, "istio", "update-base-version-master",
		"Update BASE_VERSION to master-2024-01-01", "body",
		false, "token",
		"https://github.com/test-org/istio", "master",
		[]string{"auto-merge"}, "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracker.createPRCalled || tracker.updatePRCalled {
		t.Error("expected no PR calls when there are no file changes")
	}
}

func TestCreateOrUpdatePR_Dryrun(t *testing.T) {
	// With dryrun=true, the GitHub client must never be initialized.
	var setupCalled bool
	origSetup := setupGithubClientFn
	origPush := pushCommitFn
	t.Cleanup(func() {
		setupGithubClientFn = origSetup
		pushCommitFn = origPush
	})
	setupGithubClientFn = func(token string) (*github.Client, context.Context, *github.User, error) {
		setupCalled = true
		return nil, nil, nil, nil
	}
	pushCommitFn = func(manifest model.Manifest, repo, branch, commitString string, dryrun bool, githubToken string, user github.User, force bool) (bool, error) {
		return true, nil // report changes so we reach the dryrun gate
	}

	err := CreateOrUpdatePR(
		model.Manifest{Version: "master"}, "istio", "update-base-version-master",
		"Update BASE_VERSION to master-2024-01-01", "body",
		true, "token", // dryrun=true
		"https://github.com/test-org/istio", "master",
		[]string{"auto-merge"}, "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if setupCalled {
		t.Error("GitHub client must not be initialized in dryrun mode")
	}
}

func TestCreateOrUpdatePR_ReleaseBranchVersion(t *testing.T) {
	// Ensure release-branch versions produce the correct stable branch name.
	server, tracker := newMockGitHubServer(t, mockServerOpts{})
	injectMocks(t, server, true)

	err := CreateOrUpdatePR(
		model.Manifest{Version: "1.31"}, "istio", "update-base-version-1-31",
		"Update BASE_VERSION to 1.31-2024-01-01", "body",
		false, "token",
		"https://github.com/test-org/istio", "release-1.31",
		[]string{"auto-merge"}, "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tracker.createPRCalled {
		t.Error("expected PR to be created for release branch")
	}
}
