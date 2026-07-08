// Package mattermost реализует минимальный клиент Mattermost Posts API,
// необходимый для публикации корневого сообщения и реплаев в тред.
package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

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

type createPostRequest struct {
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
	RootID    string `json:"root_id,omitempty"`
}

type createPostResponse struct {
	ID string `json:"id"`
}

// CreatePost публикует обычное (нетредированное) сообщение в канал
// и возвращает ID созданного поста.
func (c *Client) CreatePost(ctx context.Context, channelID, message string) (string, error) {
	return c.createPost(ctx, createPostRequest{ChannelID: channelID, Message: message})
}

// CreateReply публикует сообщение реплаем в тред поста rootID
// и возвращает ID созданного поста.
func (c *Client) CreateReply(ctx context.Context, channelID, rootID, message string) (string, error) {
	return c.createPost(ctx, createPostRequest{ChannelID: channelID, Message: message, RootID: rootID})
}

func (c *Client) createPost(ctx context.Context, body createPostRequest) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("формирование тела запроса: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v4/posts", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("формирование запроса создания поста: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("запрос создания поста: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Mattermost API вернул %d при создании поста", resp.StatusCode)
	}

	var result createPostResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("разбор ответа создания поста: %w", err)
	}

	return result.ID, nil
}
