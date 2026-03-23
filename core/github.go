package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GithubPR represents a pull request or issue from GitHub.
type GithubPR struct {
	Title  string `json:"title"`
	URL    string `json:"html_url"`
	State  string `json:"state"`
	Number int    `json:"number"`
	Repo   string `json:"repo,omitempty"`
}

// GithubData holds all fetched GitHub data for a widget.
type GithubData struct {
	PRs    []GithubPR `json:"prs"`
	Issues []GithubPR `json:"issues"`
	User   string     `json:"user"`
}

func githubRequest(token, url string) (*http.Response, error) {
	client := http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	return client.Do(req)
}

// FetchGithubData fetches PRs and/or issues for the authenticated user.
func FetchGithubData(token string, showPRs, showIssues bool) (*GithubData, error) {
	if token == "" {
		return nil, fmt.Errorf("github: no token configured")
	}

	result := &GithubData{}

	// Get current user
	resp, err := githubRequest(token, "https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("github: user fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github: user status %s", resp.Status)
	}
	var user struct {
		Login string `json:"login"`
	}
	json.NewDecoder(resp.Body).Decode(&user)
	result.User = user.Login

	type ghItem struct {
		Title       string `json:"title"`
		HTMLURL     string `json:"html_url"`
		State       string `json:"state"`
		Number      int    `json:"number"`
		PullRequest *struct{} `json:"pull_request"`
		Repo        struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}

	if showPRs {
		resp2, err := githubRequest(token, "https://api.github.com/search/issues?q=is:open+is:pr+author:"+user.Login+"&per_page=10")
		if err == nil && resp2.StatusCode == 200 {
			var res struct {
				Items []ghItem `json:"items"`
			}
			json.NewDecoder(resp2.Body).Decode(&res)
			resp2.Body.Close()
			for _, it := range res.Items {
				result.PRs = append(result.PRs, GithubPR{
					Title:  it.Title,
					URL:    it.HTMLURL,
					State:  it.State,
					Number: it.Number,
					Repo:   it.Repo.FullName,
				})
			}
		} else if resp2 != nil {
			resp2.Body.Close()
		}
	}

	if showIssues {
		resp3, err := githubRequest(token, "https://api.github.com/search/issues?q=is:open+is:issue+assignee:"+user.Login+"&per_page=10")
		if err == nil && resp3.StatusCode == 200 {
			var res struct {
				Items []ghItem `json:"items"`
			}
			json.NewDecoder(resp3.Body).Decode(&res)
			resp3.Body.Close()
			for _, it := range res.Items {
				result.Issues = append(result.Issues, GithubPR{
					Title:  it.Title,
					URL:    it.HTMLURL,
					State:  it.State,
					Number: it.Number,
					Repo:   it.Repo.FullName,
				})
			}
		} else if resp3 != nil {
			resp3.Body.Close()
		}
	}

	return result, nil
}
