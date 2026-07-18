package githubapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
)

type repositoryHTTPRequest struct {
	method  string
	url     string
	headers map[string]string
}

type repositoryHTTP struct {
	requests []repositoryHTTPRequest
	do       func(method, rawURL string, headers map[string]string) ([]byte, int, error)
}

func (f *repositoryHTTP) Do(_ context.Context, method, rawURL string, headers map[string]string, _ []byte) ([]byte, int, error) {
	f.requests = append(f.requests, repositoryHTTPRequest{method: method, url: rawURL, headers: headers})
	return f.do(method, rawURL, headers)
}

func repositoryPage(t *testing.T, total int, names []string, extra map[string]any) []byte {
	t.Helper()
	rows := make([]map[string]any, 0, len(names))
	for _, name := range names {
		row := map[string]any{"full_name": name}
		for key, value := range extra {
			row[key] = value
		}
		rows = append(rows, row)
	}
	body, err := json.Marshal(map[string]any{"total_count": total, "repositories": rows})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestRepositoryInstallationReportsBoundedContentsMaximum(t *testing.T) {
	keyPEM, _ := testKeyPEM(t)
	for _, tt := range []struct {
		name, permission, want string
		err                    bool
	}{
		{name: "none", want: "none"},
		{name: "read", permission: "read", want: "read"},
		{name: "write", permission: "write", want: "write"},
		{name: "invalid", permission: "admin", err: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			permissions := map[string]string{}
			if tt.permission != "" {
				permissions["contents"] = tt.permission
			}
			body, _ := json.Marshal(map[string]any{
				"id": 99, "app_id": 42,
				"account":     map[string]any{"login": "Acme", "type": "Organization"},
				"permissions": permissions,
			})
			client := New(&fakeHTTP{respStatus: 200, respBody: body}, "https://api.example.test")
			owner, maximum, err := client.RepositoryInstallation(context.Background(), 42, 99, keyPEM)
			if tt.err {
				if err == nil || !strings.Contains(err.Error(), ErrRepositoryListInvalid.Error()) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil || owner != "Acme" || maximum != tt.want {
				t.Fatalf("owner=%q maximum=%q err=%v", owner, maximum, err)
			}
		})
	}
}

func TestDiscoveryMintPayloadHasOnlyMetadataReadAndNoRepositorySelector(t *testing.T) {
	keyPEM, _ := testKeyPEM(t)
	fake := &fakeHTTP{
		respStatus: 201,
		respBody:   []byte(`{"token":"ghs_x","expires_at":"2026-07-18T01:00:00Z"}`),
	}
	_, err := New(fake, "https://api.example.test").MintToken(context.Background(), 42, 99, keyPEM, MintRequest{
		Permissions: map[string]string{"metadata": "read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(fake.body, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 1 {
		t.Fatalf("mint payload has extra fields: %#v", payload)
	}
	if !reflect.DeepEqual(payload["permissions"], map[string]any{"metadata": "read"}) {
		t.Fatalf("permissions = %#v", payload["permissions"])
	}
	for _, forbidden := range []string{"repositories", "repository_ids"} {
		if _, exists := payload[forbidden]; exists {
			t.Fatalf("mint payload included %s: %#v", forbidden, payload)
		}
	}
}

func TestRepositoryDiscoveryMintUncertaintyIsTypedAndValueFree(t *testing.T) {
	keyPEM, _ := testKeyPEM(t)
	for _, fake := range []*fakeHTTP{
		{respErr: errors.New("RAW_TRANSPORT_SECRET")},
		{respStatus: 500, respBody: []byte(`{"token":"RAW_BODY_SECRET"}`)},
		{respStatus: 201, respBody: []byte(`{"token":"ghs_cleanup","expires_at":"invalid"}`)},
	} {
		token, err := New(fake, "https://api.example.test").MintToken(context.Background(), 42, 99, keyPEM, MintRequest{
			Permissions: map[string]string{"metadata": "read"},
		})
		if !errors.Is(err, ErrTokenMintUncertain) {
			t.Fatalf("error = %v, want ErrTokenMintUncertain", err)
		}
		if fake.respStatus == 201 && (token == nil || token.Token != "ghs_cleanup") {
			t.Fatalf("cleanup token = %#v", token)
		}
		for _, forbidden := range []string{"RAW_TRANSPORT_SECRET", "RAW_BODY_SECRET", "ghs_cleanup"} {
			if strings.Contains(err.Error(), forbidden) {
				t.Fatalf("uncertain error leaked %q: %v", forbidden, err)
			}
		}
	}
}

func TestListRepositoriesPaginatesValidatesAndSorts(t *testing.T) {
	all := make([]string, 0, 101)
	for i := 100; i >= 0; i-- {
		all = append(all, fmt.Sprintf("Acme/repo-%03d", i))
	}
	fake := &repositoryHTTP{}
	fake.do = func(method, rawURL string, headers map[string]string) ([]byte, int, error) {
		if method != http.MethodGet {
			t.Fatalf("method = %q", method)
		}
		if headers["Authorization"] != "Bearer ghs_DISCOVERY_SECRET" {
			t.Fatalf("authorization = %q", headers["Authorization"])
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			t.Fatal(err)
		}
		if u.Path != "/installation/repositories" || u.Query().Get("per_page") != "100" {
			t.Fatalf("request = %s", rawURL)
		}
		switch u.Query().Get("page") {
		case "1":
			return repositoryPage(t, 101, all[:100], map[string]any{"private": true, "html_url": "https://forbidden.invalid"}), 200, nil
		case "2":
			return repositoryPage(t, 101, all[100:], nil), 200, nil
		default:
			t.Fatalf("unexpected page request: %s", rawURL)
			return nil, 0, nil
		}
	}

	got, err := New(fake, "https://api.example.test").ListRepositories(context.Background(), "ghs_DISCOVERY_SECRET", "acme")
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(got.Repositories) != 101 {
		t.Fatalf("repository count = %d, want 101", len(got.Repositories))
	}
	want := append([]string(nil), all...)
	sort.Slice(want, func(i, j int) bool {
		li, lj := strings.ToLower(want[i]), strings.ToLower(want[j])
		if li == lj {
			return want[i] < want[j]
		}
		return li < lj
	})
	if !reflect.DeepEqual(got.Repositories, want) {
		t.Fatalf("repositories not sorted/complete: got first=%q last=%q", got.Repositories[0], got.Repositories[len(got.Repositories)-1])
	}
	if len(fake.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(fake.requests))
	}
}

func TestListRepositoriesBoundaryCounts(t *testing.T) {
	for _, total := range []int{1, 100, 10_000} {
		t.Run(fmt.Sprintf("count_%d", total), func(t *testing.T) {
			fake := &repositoryHTTP{}
			fake.do = func(_, rawURL string, _ map[string]string) ([]byte, int, error) {
				u, err := url.Parse(rawURL)
				if err != nil {
					t.Fatal(err)
				}
				pageNumber := 0
				if _, err := fmt.Sscanf(u.Query().Get("page"), "%d", &pageNumber); err != nil {
					t.Fatal(err)
				}
				start := (pageNumber - 1) * repositoriesPerPage
				end := start + repositoriesPerPage
				if end > total {
					end = total
				}
				names := make([]string, 0, end-start)
				for i := start; i < end; i++ {
					names = append(names, fmt.Sprintf("acme/repo-%05d", i))
				}
				return repositoryPage(t, total, names, nil), 200, nil
			}
			got, err := New(fake, "https://api.example.test").ListRepositories(context.Background(), "ghs_x", "acme")
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Repositories) != total {
				t.Fatalf("repositories = %d, want %d", len(got.Repositories), total)
			}
			wantRequests := (total + repositoriesPerPage - 1) / repositoriesPerPage
			if len(fake.requests) != wantRequests {
				t.Fatalf("requests = %d, want %d", len(fake.requests), wantRequests)
			}
		})
	}
}

func TestListRepositoriesZeroStillValidatesOnePage(t *testing.T) {
	fake := &repositoryHTTP{do: func(_, _ string, _ map[string]string) ([]byte, int, error) {
		return repositoryPage(t, 0, nil, nil), 200, nil
	}}
	got, err := New(fake, "https://api.example.test").ListRepositories(context.Background(), "ghs_x", "acme")
	if err != nil {
		t.Fatalf("ListRepositories zero: %v", err)
	}
	if got.Repositories == nil || len(got.Repositories) != 0 {
		t.Fatalf("repositories = %#v, want non-nil empty", got.Repositories)
	}
	if len(fake.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(fake.requests))
	}
}

func TestListRepositoriesRejectsUntrustedProviderShapes(t *testing.T) {
	tests := []struct {
		name  string
		body  []byte
		limit bool
	}{
		{name: "over limit", body: repositoryPage(t, maxRepositories+1, nil, nil), limit: true},
		{name: "body limit", body: bytes.Repeat([]byte{'x'}, maxRepositoryPageBytes+1), limit: true},
		{name: "wrong owner", body: repositoryPage(t, 1, []string{"other/repo"}, nil)},
		{name: "invalid selector", body: repositoryPage(t, 1, []string{"acme/repo/extra"}, nil)},
		{name: "case folded duplicate", body: repositoryPage(t, 2, []string{"acme/Repo", "ACME/repo"}, nil)},
		{name: "short first page", body: repositoryPage(t, 101, []string{"acme/only-one"}, nil)},
		{name: "wrong types", body: []byte(`{"total_count":"one","repositories":{}}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &repositoryHTTP{do: func(_, _ string, _ map[string]string) ([]byte, int, error) {
				return tt.body, 200, nil
			}}
			_, err := New(fake, "https://api.example.test").ListRepositories(context.Background(), "ghs_x", "acme")
			if err == nil {
				t.Fatal("unexpected success")
			}
			want := ErrRepositoryListInvalid
			if tt.limit {
				want = ErrRepositoryListLimit
			}
			if !strings.Contains(err.Error(), want.Error()) {
				t.Fatalf("error = %v, want class %v", err, want)
			}
		})
	}
}

func TestRepositoryDiscoveryHTTPDoesNotFollowRedirects(t *testing.T) {
	var targetHit atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		targetHit.Store(true)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer source.Close()

	_, err := New(NewHTTP(), source.URL).ListRepositories(context.Background(), "ghs_REDIRECT_SECRET", "acme")
	if !errors.Is(err, ErrRepositoryListUnavailable) {
		t.Fatalf("redirect error = %v", err)
	}
	if targetHit.Load() {
		t.Fatal("repository discovery followed a provider redirect")
	}
}

func TestListRepositoriesErrorsNeverLeakTokenOrBody(t *testing.T) {
	fake := &repositoryHTTP{do: func(_, _ string, _ map[string]string) ([]byte, int, error) {
		return []byte(`{"message":"RAW_PROVIDER_SECRET"}`), 500, nil
	}}
	_, err := New(fake, "https://api.example.test").ListRepositories(context.Background(), "ghs_DISCOVERY_SECRET", "acme")
	if err == nil {
		t.Fatal("unexpected success")
	}
	for _, forbidden := range []string{"ghs_DISCOVERY_SECRET", "RAW_PROVIDER_SECRET", "api.example.test"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("error leaked %q: %v", forbidden, err)
		}
	}
}
