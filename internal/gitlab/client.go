// Package gitlab реализует минимальный клиент GitLab REST API v4,
// необходимый для получения открытых merge request'ов, approvals и notes.
package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const perPage = 100

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// projectPath — путь до проекта вида "group/project" или "group/subgroup/project".
func projectID(projectPath string) string {
	return url.PathEscape(projectPath)
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out interface{}) error {
	u := c.baseURL + "/api/v4" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("формирование запроса %s: %w", path, err)
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("запрос %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitLab API вернул %d для %s", resp.StatusCode, path)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("разбор ответа %s: %w", path, err)
	}

	return nil
}

// ListOpenMergeRequests возвращает все МР проекта в состоянии opened, обходя пагинацию.
func (c *Client) ListOpenMergeRequests(ctx context.Context, projectPath string) ([]MergeRequest, error) {
	var all []MergeRequest
	page := 1

	for {
		var pageResult []MergeRequest
		query := url.Values{
			"state":    {"opened"},
			"per_page": {fmt.Sprintf("%d", perPage)},
			"page":     {fmt.Sprintf("%d", page)},
		}

		path := fmt.Sprintf("/projects/%s/merge_requests", projectID(projectPath))
		if err := c.get(ctx, path, query, &pageResult); err != nil {
			return nil, fmt.Errorf("получение списка МР для %s: %w", projectPath, err)
		}

		all = append(all, pageResult...)

		if len(pageResult) < perPage {
			break
		}
		page++
	}

	return all, nil
}

// GetApprovals возвращает уникальные имена пользователей, поставивших approve на МР.
func (c *Client) GetApprovals(ctx context.Context, projectPath string, mrIID int) ([]string, error) {
	var result approvalsResponse

	path := fmt.Sprintf("/projects/%s/merge_requests/%d/approvals", projectID(projectPath), mrIID)
	if err := c.get(ctx, path, nil, &result); err != nil {
		return nil, fmt.Errorf("получение approvals для %s !%d: %w", projectPath, mrIID, err)
	}

	usernames := make([]string, 0, len(result.ApprovedBy))
	for _, a := range result.ApprovedBy {
		usernames = append(usernames, a.User.Username)
	}

	return usernames, nil
}

// ListNotes возвращает все notes (комментарии и системные заметки) МР, обходя пагинацию.
func (c *Client) ListNotes(ctx context.Context, projectPath string, mrIID int) ([]Note, error) {
	var all []Note
	page := 1

	for {
		var pageResult []Note
		query := url.Values{
			"per_page": {fmt.Sprintf("%d", perPage)},
			"page":     {fmt.Sprintf("%d", page)},
		}

		path := fmt.Sprintf("/projects/%s/merge_requests/%d/notes", projectID(projectPath), mrIID)
		if err := c.get(ctx, path, query, &pageResult); err != nil {
			return nil, fmt.Errorf("получение notes для %s !%d: %w", projectPath, mrIID, err)
		}

		all = append(all, pageResult...)

		if len(pageResult) < perPage {
			break
		}
		page++
	}

	return all, nil
}
