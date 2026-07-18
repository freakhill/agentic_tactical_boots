package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/freakhill/safeslop/internal/engine/creds/githubapp"
	"github.com/freakhill/safeslop/internal/engine/secrets"
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

func runRepositoryDiscovery(_ context.Context, account string, _ repositoryDiscoveryRuntime) jsoncontract.Envelope {
	return jsoncontract.Error(jsoncontract.NewMessage(
		jsoncontract.CodeInternal,
		"GitHub repository discovery not implemented",
		false,
		map[string]any{"account": account},
	))
}

func cmdCredsRepositoriesWithDeps(d *dependencies) *cobra.Command {
	var output string
	c := &cobra.Command{
		Use:   "repositories <host>/<owner> --output json",
		Short: "List repositories visible to one linked GitHub App installation",
		Args:  cobra.ExactArgs(1),
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
