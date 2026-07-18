package githubapp

import (
	"context"
	"errors"
)

const (
	repositoriesPerPage = 100
	maxRepositoryPages  = 100
	maxRepositories     = repositoriesPerPage * maxRepositoryPages
)

var (
	ErrRepositoryListInvalid = errors.New("github repository list response is invalid")
	ErrRepositoryListLimit   = errors.New("github repository list exceeds safety limits")
)

// RepositoryList is the minimum validated installation repository snapshot.
type RepositoryList struct {
	Repositories []string
}

// RepositoryInstallation returns the installation owner and current maximum Contents access.
func (c *Client) RepositoryInstallation(_ context.Context, _, _ int, _ []byte) (string, string, error) {
	return "", "", errors.New("github repository discovery not implemented")
}

// ListRepositories returns a complete, bounded installation repository snapshot.
func (c *Client) ListRepositories(_ context.Context, _ string, _ string) (RepositoryList, error) {
	return RepositoryList{}, errors.New("github repository discovery not implemented")
}
