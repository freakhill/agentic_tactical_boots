package githubapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

const (
	repositoriesPerPage     = 100
	maxRepositoryPages      = 100
	maxRepositories         = repositoriesPerPage * maxRepositoryPages
	maxRepositoryPageBytes  = 4 << 20
	maxRepositoryNamesBytes = 4 << 20
)

var (
	ErrRepositoryListInvalid     = errors.New("github repository list response is invalid")
	ErrRepositoryListLimit       = errors.New("github repository list exceeds safety limits")
	ErrRepositoryListUnavailable = errors.New("github repository list is unavailable")
	repositorySelectorRE         = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)
)

// RepositoryList is the minimum validated installation repository snapshot.
type RepositoryList struct {
	Repositories []string
}

// RepositoryInstallation returns the installation owner and current maximum Contents access.
func (c *Client) RepositoryInstallation(ctx context.Context, appID, instID int, keyPEM []byte) (string, string, error) {
	inst, err := c.InstallationInfo(ctx, appID, instID, keyPEM)
	if err != nil {
		return "", "", err
	}
	owner := inst.AccountLogin()
	if owner == "" {
		return "", "", fmt.Errorf("%w: missing installation owner", ErrRepositoryListInvalid)
	}
	maximum := "none"
	if contents, ok := inst.Permissions["contents"]; ok {
		if contents != "read" && contents != "write" {
			return "", "", fmt.Errorf("%w: invalid Contents permission", ErrRepositoryListInvalid)
		}
		maximum = contents
	}
	return owner, maximum, nil
}

// ListRepositories returns a complete, bounded installation repository snapshot. It follows only
// fixed integer pages on the public installation endpoint; provider-supplied links/cursors and all
// repository fields except full_name are ignored.
func (c *Client) ListRepositories(ctx context.Context, token, expectedOwner string) (RepositoryList, error) {
	type repositoryRow struct {
		FullName string `json:"full_name"`
	}
	type repositoryPage struct {
		TotalCount   *int            `json:"total_count"`
		Repositories []repositoryRow `json:"repositories"`
	}

	out := make([]string, 0)
	seen := make(map[string]struct{})
	totalNameBytes := 0
	expectedTotal := -1
	expectedPages := 1

	for pageNumber := 1; pageNumber <= expectedPages; pageNumber++ {
		query := url.Values{}
		query.Set("page", fmt.Sprintf("%d", pageNumber))
		query.Set("per_page", fmt.Sprintf("%d", repositoriesPerPage))
		endpoint := c.apiBase + "/installation/repositories?" + query.Encode()
		body, status, err := c.http.Do(ctx, http.MethodGet, endpoint, installationHeaders(token), nil)
		if err != nil {
			if ctx.Err() != nil {
				return RepositoryList{}, ctx.Err()
			}
			return RepositoryList{}, ErrRepositoryListUnavailable
		}
		if status/100 != 2 {
			return RepositoryList{}, ErrRepositoryListUnavailable
		}
		if len(body) > maxRepositoryPageBytes {
			return RepositoryList{}, ErrRepositoryListLimit
		}
		var page repositoryPage
		if err := json.Unmarshal(body, &page); err != nil || page.TotalCount == nil || page.Repositories == nil {
			return RepositoryList{}, ErrRepositoryListInvalid
		}
		if *page.TotalCount < 0 {
			return RepositoryList{}, ErrRepositoryListInvalid
		}
		if *page.TotalCount > maxRepositories {
			return RepositoryList{}, ErrRepositoryListLimit
		}
		if pageNumber == 1 {
			expectedTotal = *page.TotalCount
			expectedPages = (expectedTotal + repositoriesPerPage - 1) / repositoriesPerPage
			if expectedPages == 0 {
				expectedPages = 1
			}
			if expectedPages > maxRepositoryPages {
				return RepositoryList{}, ErrRepositoryListLimit
			}
		} else if *page.TotalCount != expectedTotal {
			return RepositoryList{}, ErrRepositoryListInvalid
		}

		expectedRows := repositoriesPerPage
		if pageNumber == expectedPages {
			expectedRows = expectedTotal - repositoriesPerPage*(expectedPages-1)
		}
		if len(page.Repositories) != expectedRows {
			return RepositoryList{}, ErrRepositoryListInvalid
		}
		for _, row := range page.Repositories {
			if !repositorySelectorRE.MatchString(row.FullName) {
				return RepositoryList{}, ErrRepositoryListInvalid
			}
			parts := strings.SplitN(row.FullName, "/", 2)
			if !strings.EqualFold(parts[0], expectedOwner) {
				return RepositoryList{}, ErrRepositoryListInvalid
			}
			canonical := strings.ToLower(row.FullName)
			if _, exists := seen[canonical]; exists {
				return RepositoryList{}, ErrRepositoryListInvalid
			}
			seen[canonical] = struct{}{}
			totalNameBytes += len(row.FullName)
			if totalNameBytes > maxRepositoryNamesBytes {
				return RepositoryList{}, ErrRepositoryListLimit
			}
			out = append(out, row.FullName)
		}
	}
	if len(out) != expectedTotal {
		return RepositoryList{}, ErrRepositoryListInvalid
	}
	sort.Slice(out, func(i, j int) bool {
		left, right := strings.ToLower(out[i]), strings.ToLower(out[j])
		if left == right {
			return out[i] < out[j]
		}
		return left < right
	})
	if out == nil {
		out = []string{}
	}
	return RepositoryList{Repositories: out}, nil
}

func installationHeaders(token string) map[string]string {
	return map[string]string{
		"Authorization":        "Bearer " + token,
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": apiVersion,
	}
}
