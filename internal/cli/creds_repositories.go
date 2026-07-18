package cli

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/freakhill/safeslop/internal/engine/creds/githubapp"
	"github.com/freakhill/safeslop/internal/engine/secrets"
	"github.com/freakhill/safeslop/internal/engine/userconfig"
	"github.com/freakhill/safeslop/internal/jsoncontract"
)

const (
	repositoryDiscoveryTimeout = 60 * time.Second
	repositoryCleanupTimeout   = 10 * time.Second
)

type githubRepositoryDiscoveryClient interface {
	RepositoryInstallation(context.Context, int, int, []byte) (string, string, error)
	MintToken(context.Context, int, int, []byte, githubapp.MintRequest) (*githubapp.Token, error)
	ListRepositories(context.Context, string, string) (githubapp.RepositoryList, error)
	Revoke(context.Context, string) error
}

type repositoryDiscoveryRuntime struct {
	accountsPath   string
	resolveSecret  func(context.Context, string) (string, error)
	client         githubRepositoryDiscoveryClient
	now            func() time.Time
	cleanupContext func() (context.Context, context.CancelFunc)
}

func defaultRepositoryCleanupContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), repositoryCleanupTimeout)
}

func repositoryDiscoveryRuntimeWithDeps(d *dependencies) (repositoryDiscoveryRuntime, error) {
	pathFn := d.repositoryAccountsPath
	if pathFn == nil {
		pathFn = accountsPathOrErr
	}
	path, err := pathFn()
	if err != nil {
		return repositoryDiscoveryRuntime{}, err
	}
	resolve := d.repositoryResolveSecret
	if resolve == nil {
		resolve = secrets.Resolve
	}
	clientFn := d.newRepositoryDiscoveryClient
	if clientFn == nil {
		clientFn = func() githubRepositoryDiscoveryClient {
			return githubapp.New(githubapp.NewHTTP(), "")
		}
	}
	cleanup := d.repositoryCleanupContext
	if cleanup == nil {
		cleanup = defaultRepositoryCleanupContext
	}
	return repositoryDiscoveryRuntime{
		accountsPath: path, resolveSecret: resolve, client: clientFn(), now: d.now,
		cleanupContext: cleanup,
	}, nil
}

func repositoryDiscoveryError(code jsoncontract.ErrorCode, message, account string) jsoncontract.Envelope {
	details := map[string]any{}
	if account != "" {
		details["account"] = account
	}
	return jsoncontract.Error(jsoncontract.NewMessage(code, message, false, details))
}

func repositoryDiscoveryCleanupUncertain(account string) jsoncontract.Envelope {
	return repositoryDiscoveryError(
		jsoncontract.CodeCredentialRevokeFailed,
		"GitHub discovery cleanup was not confirmed; no repositories were imported and a metadata-only credential may remain usable for at most one hour",
		account,
	)
}

func repositoryDiscoveryAccount(account string) (string, error) {
	parts := strings.Split(account, "/")
	if len(parts) != 2 || parts[0] != "github.com" || parts[1] == "" ||
		!profileCredentialRepoComponentRE.MatchString(parts[1]) {
		return "", errors.New("invalid public GitHub account selector")
	}
	return parts[1], nil
}

func runRepositoryDiscovery(ctx context.Context, account string, runtime repositoryDiscoveryRuntime) jsoncontract.Envelope {
	owner, err := repositoryDiscoveryAccount(account)
	if err != nil {
		return repositoryDiscoveryError(jsoncontract.CodeInvalidArgument, "GitHub account must be github.com/owner", account)
	}
	accounts, err := userconfig.LoadAccounts(runtime.accountsPath)
	if err != nil {
		return repositoryDiscoveryError(jsoncontract.CodeIOError, "load GitHub account links", account)
	}
	link := accounts.Lookup("github.com", owner)
	if link == nil || link.Forge != "github" || link.Github == nil || link.Host != "github.com" || !strings.EqualFold(link.Owner, owner) {
		return repositoryDiscoveryError(jsoncontract.CodeNotFound, "linked public GitHub App account not found", account)
	}
	keyValue, err := runtime.resolveSecret(ctx, link.Github.PrivateKeyRef)
	if err != nil {
		return repositoryDiscoveryError(jsoncontract.CodeAuthRequired, "linked GitHub App key is unavailable", account)
	}
	keyPEM := []byte(keyValue)
	keyValue = ""
	defer clear(keyPEM)

	installationOwner, maximum, err := runtime.client.RepositoryInstallation(
		ctx, link.Github.AppID, link.Github.InstallationID, keyPEM,
	)
	if err != nil {
		code := jsoncontract.CodeIOError
		if errors.Is(err, githubapp.ErrRepositoryListInvalid) {
			code = jsoncontract.CodeSchemaViolation
		}
		return repositoryDiscoveryError(code, "verify linked GitHub App installation", account)
	}
	if !strings.EqualFold(installationOwner, owner) || (maximum != "none" && maximum != "read" && maximum != "write") {
		return repositoryDiscoveryError(jsoncontract.CodeSchemaViolation, "GitHub App installation metadata is invalid", account)
	}

	token, err := runtime.client.MintToken(ctx, link.Github.AppID, link.Github.InstallationID, keyPEM, githubapp.MintRequest{
		Permissions: map[string]string{"metadata": "read"},
	})
	cleanupToken := func(value string) error {
		cleanupCtx, cancelCleanup := runtime.cleanupContext()
		defer cancelCleanup()
		return runtime.client.Revoke(cleanupCtx, value)
	}
	if err != nil {
		if token != nil && token.Token != "" {
			if cleanupToken(token.Token) != nil {
				return repositoryDiscoveryCleanupUncertain(account)
			}
			return repositoryDiscoveryError(jsoncontract.CodeSchemaViolation, "GitHub repository discovery credential response is invalid", account)
		}
		if errors.Is(err, githubapp.ErrTokenMintUncertain) {
			return repositoryDiscoveryCleanupUncertain(account)
		}
		return repositoryDiscoveryError(jsoncontract.CodeIOError, "mint GitHub repository discovery credential", account)
	}

	if token == nil || token.Token == "" {
		return repositoryDiscoveryCleanupUncertain(account)
	}
	primary := jsoncontract.Envelope{}
	now := runtime.now()
	if !token.ExpiresAt.After(now) || token.ExpiresAt.After(now.Add(time.Hour)) {
		primary = repositoryDiscoveryError(jsoncontract.CodeSchemaViolation, "GitHub discovery credential lifetime is invalid", account)
	} else {
		list, listErr := runtime.client.ListRepositories(ctx, token.Token, owner)
		switch {
		case listErr == nil:
			primary = jsoncontract.OK(map[string]any{
				"account":          account,
				"contents_maximum": maximum,
				"repositories":     list.Repositories,
			})
		case errors.Is(listErr, githubapp.ErrRepositoryListLimit), errors.Is(listErr, githubapp.ErrRepositoryListInvalid):
			primary = repositoryDiscoveryError(jsoncontract.CodeSchemaViolation, "GitHub repository snapshot is invalid or exceeds safety limits", account)
		case errors.Is(listErr, context.DeadlineExceeded):
			primary = repositoryDiscoveryError(jsoncontract.CodeTimeout, "GitHub repository discovery timed out", account)
		default:
			primary = repositoryDiscoveryError(jsoncontract.CodeIOError, "GitHub repository discovery did not complete", account)
		}
	}

	if cleanupToken(token.Token) != nil {
		return repositoryDiscoveryCleanupUncertain(account)
	}
	return primary
}

func cmdCredsRepositoriesWithDeps(d *dependencies) *cobra.Command {
	var output string
	c := &cobra.Command{
		Use:   "repositories <host>/<owner> --output json",
		Short: "List repositories visible to one linked GitHub App installation",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return emitContractError(jsoncontract.CodeInvalidArgument, "creds repositories requires exactly one github.com/owner account", map[string]any{})
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if output != "json" {
				return emitContractError(jsoncontract.CodeInvalidArgument, "creds repositories requires --output json", map[string]any{})
			}
			runtime, err := repositoryDiscoveryRuntimeWithDeps(d)
			if err != nil {
				return emitContractError(jsoncontract.CodeIOError, "load GitHub account links", map[string]any{})
			}
			signalCtx, stopSignals := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stopSignals()
			requestCtx, cancel := context.WithTimeout(signalCtx, repositoryDiscoveryTimeout)
			defer cancel()
			env := runRepositoryDiscovery(requestCtx, args[0], runtime)
			emitContract(env)
			if !env.OK {
				return errOutputEmitted
			}
			return nil
		},
	}
	c.Flags().StringVar(&output, "output", "", "output format: json")
	return c
}
