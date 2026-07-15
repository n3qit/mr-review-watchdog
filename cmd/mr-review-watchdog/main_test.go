package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/n3qit/mr-review-watchdog/internal/checker"
	"github.com/n3qit/mr-review-watchdog/internal/config"
	"github.com/n3qit/mr-review-watchdog/internal/gitlab"
)

// fakeGitLabClient возвращает заранее заданные МР для одного репозитория.
type fakeGitLabClient struct {
	mrs []gitlab.MergeRequest
}

func (f *fakeGitLabClient) ListOpenMergeRequests(ctx context.Context, projectPath string) ([]gitlab.MergeRequest, error) {
	return f.mrs, nil
}

func (f *fakeGitLabClient) GetApprovals(ctx context.Context, projectPath string, mrIID int) ([]string, error) {
	return nil, nil
}

func (f *fakeGitLabClient) ListDiscussions(ctx context.Context, projectPath string, mrIID int) ([]gitlab.Discussion, error) {
	return nil, nil
}

func (f *fakeGitLabClient) ListGroupMembers(ctx context.Context, groupPath string) ([]string, error) {
	return nil, nil
}

// fakeGitLabClientWithError возвращает ошибку при получении списка МР.
type fakeGitLabClientWithError struct{}

func (f *fakeGitLabClientWithError) ListOpenMergeRequests(ctx context.Context, projectPath string) ([]gitlab.MergeRequest, error) {
	return nil, fmt.Errorf("502 bad gateway")
}

func (f *fakeGitLabClientWithError) GetApprovals(ctx context.Context, projectPath string, mrIID int) ([]string, error) {
	return nil, nil
}

func (f *fakeGitLabClientWithError) ListDiscussions(ctx context.Context, projectPath string, mrIID int) ([]gitlab.Discussion, error) {
	return nil, nil
}

func (f *fakeGitLabClientWithError) ListGroupMembers(ctx context.Context, groupPath string) ([]string, error) {
	return nil, nil
}

// fakeGitLabClientTeamFetchError возвращает ошибку только при получении участников группы.
type fakeGitLabClientTeamFetchError struct{}

func (f *fakeGitLabClientTeamFetchError) ListOpenMergeRequests(ctx context.Context, projectPath string) ([]gitlab.MergeRequest, error) {
	return nil, nil
}

func (f *fakeGitLabClientTeamFetchError) GetApprovals(ctx context.Context, projectPath string, mrIID int) ([]string, error) {
	return nil, nil
}

func (f *fakeGitLabClientTeamFetchError) ListDiscussions(ctx context.Context, projectPath string, mrIID int) ([]gitlab.Discussion, error) {
	return nil, nil
}

func (f *fakeGitLabClientTeamFetchError) ListGroupMembers(ctx context.Context, groupPath string) ([]string, error) {
	return nil, fmt.Errorf("502 bad gateway")
}

type postCall struct{ channelID, message string }
type replyCall struct{ channelID, rootID, message string }

type mockMattermostClient struct {
	postCalls  []postCall
	replyCalls []replyCall
	nextPostID int
}

func (m *mockMattermostClient) CreatePost(ctx context.Context, channelID, message string) (string, error) {
	m.postCalls = append(m.postCalls, postCall{channelID, message})
	m.nextPostID++
	return fmt.Sprintf("post-%d", m.nextPostID), nil
}

func (m *mockMattermostClient) CreateReply(ctx context.Context, channelID, rootID, message string) (string, error) {
	m.replyCalls = append(m.replyCalls, replyCall{channelID, rootID, message})
	return "reply-1", nil
}

func testConfig() *config.Config {
	return &config.Config{
		Mattermost:   config.MattermostConfig{ChannelID: "chan1"},
		Check:        config.CheckConfig{MinReviewers: 2, MinAgeHours: 24},
		Repositories: []string{"group/project"},
	}
}

func TestRun_ProblemsAndErrors_ReplyWithRootID(t *testing.T) {
	now := time.Now()
	gl := &fakeGitLabClient{
		mrs: []gitlab.MergeRequest{
			{IID: 1, Title: "needs review", CreatedAt: now.Add(-48 * time.Hour)},
		},
	}
	cfg := testConfig()
	cfg.Repositories = []string{"group/project", "group/broken"}
	mm := &mockMattermostClient{}

	gitlabWithBrokenRepo := &multiRepoFake{
		byRepo: map[string]checker.GitLabClient{
			"group/project": gl,
			"group/broken":  &fakeGitLabClientWithError{},
		},
	}

	if err := run(context.Background(), gitlabWithBrokenRepo, mm, cfg, now); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if len(mm.postCalls) != 1 {
		t.Fatalf("expected 1 CreatePost call (root message), got %d", len(mm.postCalls))
	}
	if len(mm.replyCalls) != 1 {
		t.Fatalf("expected 1 CreateReply call (errors as thread), got %d", len(mm.replyCalls))
	}
	wantRootID := "post-1"
	if mm.replyCalls[0].rootID != wantRootID {
		t.Errorf("reply root_id = %q, want %q", mm.replyCalls[0].rootID, wantRootID)
	}
}

// multiRepoFake маршрутизирует вызовы к разным фейковым клиентам по имени репозитория.
type multiRepoFake struct {
	byRepo map[string]checker.GitLabClient
}

func (m *multiRepoFake) ListOpenMergeRequests(ctx context.Context, projectPath string) ([]gitlab.MergeRequest, error) {
	return m.byRepo[projectPath].ListOpenMergeRequests(ctx, projectPath)
}

func (m *multiRepoFake) GetApprovals(ctx context.Context, projectPath string, mrIID int) ([]string, error) {
	return m.byRepo[projectPath].GetApprovals(ctx, projectPath, mrIID)
}

func (m *multiRepoFake) ListDiscussions(ctx context.Context, projectPath string, mrIID int) ([]gitlab.Discussion, error) {
	return m.byRepo[projectPath].ListDiscussions(ctx, projectPath, mrIID)
}

func (m *multiRepoFake) ListGroupMembers(ctx context.Context, groupPath string) ([]string, error) {
	for _, c := range m.byRepo {
		return c.ListGroupMembers(ctx, groupPath)
	}
	return nil, nil
}

func TestRun_OnlyErrors_PlainPostWithoutRootID(t *testing.T) {
	now := time.Now()
	cfg := testConfig()
	cfg.Repositories = []string{"group/broken"}
	mm := &mockMattermostClient{}

	if err := run(context.Background(), &fakeGitLabClientWithError{}, mm, cfg, now); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if len(mm.replyCalls) != 0 {
		t.Fatalf("expected 0 CreateReply calls when there is no root post, got %d", len(mm.replyCalls))
	}
	if len(mm.postCalls) != 1 {
		t.Fatalf("expected 1 CreatePost call (errors as plain message), got %d", len(mm.postCalls))
	}
}

func TestRun_TeamFetchError_IsFatalAndSendsNoMessages(t *testing.T) {
	now := time.Now()
	cfg := testConfig()
	cfg.GitLab.TeamGroup = "group/our-team"
	mm := &mockMattermostClient{}

	err := run(context.Background(), &fakeGitLabClientTeamFetchError{}, mm, cfg, now)
	if err == nil {
		t.Fatal("expected fatal error when team member fetch fails, got nil")
	}

	if len(mm.postCalls) != 0 || len(mm.replyCalls) != 0 {
		t.Fatalf("expected no Mattermost calls on fatal error, got postCalls=%d replyCalls=%d", len(mm.postCalls), len(mm.replyCalls))
	}
}

func TestRun_NoProblemsNoErrors_NoMessagesSent(t *testing.T) {
	now := time.Now()
	gl := &fakeGitLabClient{mrs: nil}
	cfg := testConfig()
	mm := &mockMattermostClient{}

	if err := run(context.Background(), gl, mm, cfg, now); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if len(mm.postCalls) != 0 || len(mm.replyCalls) != 0 {
		t.Fatalf("expected no Mattermost calls, got postCalls=%d replyCalls=%d", len(mm.postCalls), len(mm.replyCalls))
	}
}
