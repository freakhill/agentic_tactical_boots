package cli

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/freakhill/safeslop/internal/engine/creds/githubapp"
	"github.com/freakhill/safeslop/internal/engine/userconfig"
	"github.com/freakhill/safeslop/internal/jsoncontract"
)

type fakeRepositoryDiscoveryClient struct {
	owner, maximum  string
	installationErr error
	mintErr         error
	listErr         error
	revokeErr       error
	token           string
	expiresAt       time.Time
	repositories    []string
	mintRequest     githubapp.MintRequest
	keyBytes        string
	revokes         int
	revokeContextOK bool
	listHook        func()
}

func (f *fakeRepositoryDiscoveryClient) RepositoryInstallation(_ context.Context, _, _ int, key []byte) (string, string, error) {
	f.keyBytes = string(key)
	return f.owner, f.maximum, f.installationErr
}

func (f *fakeRepositoryDiscoveryClient) MintToken(_ context.Context, _, _ int, _ []byte, req githubapp.MintRequest) (*githubapp.Token, error) {
	f.mintRequest = req
	if f.mintErr != nil {
		return nil, f.mintErr
	}
	return &githubapp.Token{Token: f.token, ExpiresAt: f.expiresAt}, nil
}

func (f *fakeRepositoryDiscoveryClient) ListRepositories(_ context.Context, token, _ string) (githubapp.RepositoryList, error) {
	if token != f.token {
		return githubapp.RepositoryList{}, errors.New("unexpected discovery token")
	}
	if f.listHook != nil {
		f.listHook()
	}
	return githubapp.RepositoryList{Repositories: append([]string(nil), f.repositories...)}, f.listErr
}

func (f *fakeRepositoryDiscoveryClient) Revoke(ctx context.Context, token string) error {
	f.revokes++
	_, hasDeadline := ctx.Deadline()
	f.revokeContextOK = ctx.Err() == nil && hasDeadline && token == f.token
	return f.revokeErr
}

func repositoryRuntimeForTest(t *testing.T, client *fakeRepositoryDiscoveryClient) repositoryDiscoveryRuntime {
	t.Helper()
	path := t.TempDir() + "/accounts.cue"
	accounts := &userconfig.Accounts{Accounts: map[string]userconfig.Account{}}
	accounts.Upsert(userconfig.Account{
		Forge: "github", Host: "github.com", Owner: "acme",
		Github: &userconfig.GithubAccount{AppID: 42, InstallationID: 99, PrivateKeyRef: "op://PRIVATE/KEY_REF"},
	})
	if err := userconfig.SaveAccounts(path, accounts); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)
	client.owner, client.maximum = "Acme", "write"
	client.token = "ghs_DISCOVERY_SECRET"
	client.expiresAt = now.Add(59 * time.Minute)
	return repositoryDiscoveryRuntime{
		accountsPath: path,
		resolveSecret: func(_ context.Context, ref string) (string, error) {
			if ref != "op://PRIVATE/KEY_REF" {
				t.Fatalf("secret ref = %q", ref)
			}
			return "-----BEGIN PRIVATE KEY-----\nPRIVATE_KEY_BYTES", nil
		},
		client: client,
		now:    func() time.Time { return now },
		cleanupContext: func() (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), repositoryCleanupTimeout)
		},
	}
}

func TestRepositoryDiscoveryExactAuthoritySharedEnvelopeAndCleanup(t *testing.T) {
	client := &fakeRepositoryDiscoveryClient{repositories: []string{"Acme/api", "Acme/web"}}
	runtime := repositoryRuntimeForTest(t, client)
	env := runRepositoryDiscovery(context.Background(), "github.com/acme", runtime)
	if !env.OK {
		t.Fatalf("discovery error: %+v", env.Errors)
	}
	if got := env.Data["account"]; got != "github.com/acme" {
		t.Fatalf("account = %#v", got)
	}
	if got := env.Data["contents_maximum"]; got != "write" {
		t.Fatalf("contents_maximum = %#v", got)
	}
	if got := env.Data["repositories"]; !reflect.DeepEqual(got, []string{"Acme/api", "Acme/web"}) {
		t.Fatalf("repositories = %#v", got)
	}
	if len(client.mintRequest.Repositories) != 0 {
		t.Fatalf("discovery mint selected repositories: %#v", client.mintRequest.Repositories)
	}
	if !reflect.DeepEqual(client.mintRequest.Permissions, map[string]string{"metadata": "read"}) {
		t.Fatalf("permissions = %#v", client.mintRequest.Permissions)
	}
	if client.revokes != 1 || !client.revokeContextOK {
		t.Fatalf("cleanup: revokes=%d contextOK=%v", client.revokes, client.revokeContextOK)
	}
	wire, err := jsoncontract.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"PRIVATE_KEY_BYTES", "op://PRIVATE/KEY_REF", "ghs_DISCOVERY_SECRET", "appID", "installationID", "privateKeyRef"} {
		if strings.Contains(string(wire), forbidden) {
			t.Fatalf("envelope leaked %q: %s", forbidden, wire)
		}
	}
}

func TestRepositoryDiscoveryCleanupUncertainWithholdsCandidates(t *testing.T) {
	client := &fakeRepositoryDiscoveryClient{
		repositories: []string{"Acme/api"},
		revokeErr:    errors.New("RAW_REVOKE_PROVIDER_BODY"),
	}
	env := runRepositoryDiscovery(context.Background(), "github.com/acme", repositoryRuntimeForTest(t, client))
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != jsoncontract.CodeCredentialRevokeFailed {
		t.Fatalf("envelope = %+v", env)
	}
	if len(env.Data) != 0 {
		t.Fatalf("cleanup failure exposed data: %#v", env.Data)
	}
	wire, _ := json.Marshal(env)
	if strings.Contains(string(wire), "RAW_REVOKE_PROVIDER_BODY") || strings.Contains(string(wire), "Acme/api") {
		t.Fatalf("cleanup failure leaked provider/candidates: %s", wire)
	}
	if client.revokes != 1 {
		t.Fatalf("revokes = %d, want 1", client.revokes)
	}
}

func TestRepositoryDiscoveryCancellationStillUsesIndependentCleanupContext(t *testing.T) {
	requestCtx, cancel := context.WithCancel(context.Background())
	client := &fakeRepositoryDiscoveryClient{listErr: context.Canceled}
	client.listHook = cancel
	env := runRepositoryDiscovery(requestCtx, "github.com/acme", repositoryRuntimeForTest(t, client))
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != jsoncontract.CodeIOError {
		t.Fatalf("envelope = %+v", env)
	}
	if requestCtx.Err() == nil {
		t.Fatal("test did not cancel request context")
	}
	if client.revokes != 1 || !client.revokeContextOK {
		t.Fatalf("cleanup inherited cancellation: revokes=%d contextOK=%v", client.revokes, client.revokeContextOK)
	}
}

func TestRepositoryDiscoveryPreMintFailureDoesNotRevoke(t *testing.T) {
	client := &fakeRepositoryDiscoveryClient{mintErr: errors.New("RAW_MINT_PROVIDER_BODY")}
	env := runRepositoryDiscovery(context.Background(), "github.com/acme", repositoryRuntimeForTest(t, client))
	if env.OK || client.revokes != 0 {
		t.Fatalf("envelope=%+v revokes=%d", env, client.revokes)
	}
	wire, _ := json.Marshal(env)
	if strings.Contains(string(wire), "RAW_MINT_PROVIDER_BODY") {
		t.Fatalf("provider error leaked: %s", wire)
	}
}

func TestRepositoryDiscoveryRejectsInvalidAccountBeforeSecretResolution(t *testing.T) {
	client := &fakeRepositoryDiscoveryClient{}
	runtime := repositoryRuntimeForTest(t, client)
	resolved := false
	runtime.resolveSecret = func(context.Context, string) (string, error) {
		resolved = true
		return "", nil
	}
	for _, account := range []string{"", "github.com", "github.example/acme", "github.com/acme/extra", "github.com/other"} {
		env := runRepositoryDiscovery(context.Background(), account, runtime)
		if env.OK {
			t.Fatalf("account %q unexpectedly accepted", account)
		}
	}
	if resolved {
		t.Fatal("invalid/missing accounts must fail before secret resolution")
	}
}

func TestRepositoryDiscoveryRejectsOverlongTokenLifetimeThenRevokes(t *testing.T) {
	client := &fakeRepositoryDiscoveryClient{}
	runtime := repositoryRuntimeForTest(t, client)
	client.expiresAt = runtime.now().Add(time.Hour + time.Second)
	env := runRepositoryDiscovery(context.Background(), "github.com/acme", runtime)
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != jsoncontract.CodeSchemaViolation {
		t.Fatalf("envelope = %+v", env)
	}
	if client.revokes != 1 {
		t.Fatalf("revokes = %d, want 1", client.revokes)
	}
}

func TestCredsRepositoriesCommandUsesExactAccountAndSharedEnvelope(t *testing.T) {
	client := &fakeRepositoryDiscoveryClient{repositories: []string{"Acme/api"}}
	runtime := repositoryRuntimeForTest(t, client)
	d := defaultDependencies()
	d.repositoryAccountsPath = func() (string, error) { return runtime.accountsPath, nil }
	d.repositoryResolveSecret = runtime.resolveSecret
	d.newRepositoryDiscoveryClient = func() githubRepositoryDiscoveryClient { return client }
	d.repositoryCleanupContext = runtime.cleanupContext
	d.now = runtime.now

	out, err := runRootForTestWithDeps(t, t.TempDir(), d, "creds", "repositories", "github.com/acme", "--output", "json")
	if err != nil {
		t.Fatalf("creds repositories: %v\nout=%s", err, out)
	}
	env := parseEnvelopeForTest(t, out)
	if !env.OK || env.Data["account"] != "github.com/acme" {
		t.Fatalf("envelope = %+v", env)
	}
}
