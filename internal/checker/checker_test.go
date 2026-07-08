package checker

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/n3qit/mr-review-watchdog/internal/gitlab"
)

type mockClient struct {
	mrs          map[string][]gitlab.MergeRequest
	mrsErr       map[string]error
	approvals    map[string][]string
	approvalsErr map[string]error
	notes        map[string][]gitlab.Note
	notesErr     map[string]error
}

func key(repo string, iid int) string {
	return fmt.Sprintf("%s#%d", repo, iid)
}

func (m *mockClient) ListOpenMergeRequests(ctx context.Context, projectPath string) ([]gitlab.MergeRequest, error) {
	if err, ok := m.mrsErr[projectPath]; ok {
		return nil, err
	}
	return m.mrs[projectPath], nil
}

func (m *mockClient) GetApprovals(ctx context.Context, projectPath string, mrIID int) ([]string, error) {
	k := key(projectPath, mrIID)
	if err, ok := m.approvalsErr[k]; ok {
		return nil, err
	}
	return m.approvals[k], nil
}

func (m *mockClient) ListNotes(ctx context.Context, projectPath string, mrIID int) ([]gitlab.Note, error) {
	k := key(projectPath, mrIID)
	if err, ok := m.notesErr[k]; ok {
		return nil, err
	}
	return m.notes[k], nil
}

func TestRun_ExcludesDraftMRs(t *testing.T) {
	now := time.Now()
	client := &mockClient{
		mrs: map[string][]gitlab.MergeRequest{
			"group/project": {
				{IID: 1, Title: "draft mr", Draft: true, CreatedAt: now.Add(-48 * time.Hour)},
				{IID: 2, Title: "legacy wip", WorkInProgress: true, CreatedAt: now.Add(-48 * time.Hour)},
			},
		},
	}

	report := Run(context.Background(), client, []string{"group/project"}, 2, 24, now)

	if len(report.ProblemMRs) != 0 {
		t.Errorf("expected 0 problem MRs, got %d", len(report.ProblemMRs))
	}
}

func TestRun_ExcludesYoungMRs(t *testing.T) {
	now := time.Now()
	client := &mockClient{
		mrs: map[string][]gitlab.MergeRequest{
			"group/project": {
				{IID: 1, Title: "fresh mr", CreatedAt: now.Add(-1 * time.Hour)},
			},
		},
	}

	report := Run(context.Background(), client, []string{"group/project"}, 2, 24, now)

	if len(report.ProblemMRs) != 0 {
		t.Errorf("expected 0 problem MRs, got %d", len(report.ProblemMRs))
	}
}

func TestRun_CountsUniqueReviewersFromApprovalsAndNotes(t *testing.T) {
	now := time.Now()
	client := &mockClient{
		mrs: map[string][]gitlab.MergeRequest{
			"group/project": {
				{
					IID:       1,
					Title:     "old enough mr",
					Author:    gitlab.User{Username: "ivanov"},
					CreatedAt: now.Add(-48 * time.Hour),
				},
			},
		},
		approvals: map[string][]string{
			key("group/project", 1): {"petrov"},
		},
		notes: map[string][]gitlab.Note{
			key("group/project", 1): {
				{Author: gitlab.User{Username: "petrov"}, System: false},  // дубликат с approvals
				{Author: gitlab.User{Username: "sidorov"}, System: false}, // новый ревьюер
				{Author: gitlab.User{Username: "ivanov"}, System: false},  // автор, исключается
				{Author: gitlab.User{Username: "bot"}, System: true},      // системная заметка, исключается
			},
		},
	}

	report := Run(context.Background(), client, []string{"group/project"}, 2, 24, now)

	if len(report.ProblemMRs) != 0 {
		t.Fatalf("expected MR to have 2 reviewers (not problem), got problems: %+v", report.ProblemMRs)
	}
}

func TestRun_ProblemMRWhenNotEnoughReviewers(t *testing.T) {
	now := time.Now()
	client := &mockClient{
		mrs: map[string][]gitlab.MergeRequest{
			"group/project": {
				{
					IID:       1,
					Title:     "needs review",
					Author:    gitlab.User{Username: "ivanov"},
					CreatedAt: now.Add(-48 * time.Hour),
				},
			},
		},
		approvals: map[string][]string{
			key("group/project", 1): {"petrov"},
		},
	}

	report := Run(context.Background(), client, []string{"group/project"}, 2, 24, now)

	if len(report.ProblemMRs) != 1 {
		t.Fatalf("expected 1 problem MR, got %d", len(report.ProblemMRs))
	}
	if len(report.ProblemMRs[0].Reviewers) != 1 {
		t.Errorf("expected 1 reviewer, got %d", len(report.ProblemMRs[0].Reviewers))
	}
}

func TestRun_RepoErrorDoesNotAbortOthers(t *testing.T) {
	now := time.Now()
	client := &mockClient{
		mrs: map[string][]gitlab.MergeRequest{
			"group/project-b": {
				{
					IID:       1,
					Title:     "needs review",
					Author:    gitlab.User{Username: "ivanov"},
					CreatedAt: now.Add(-48 * time.Hour),
				},
			},
		},
		mrsErr: map[string]error{
			"group/project-a": errors.New("502 bad gateway"),
		},
	}

	report := Run(context.Background(), client, []string{"group/project-a", "group/project-b"}, 2, 24, now)

	if len(report.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %+v", len(report.Errors), report.Errors)
	}
	if len(report.ProblemMRs) != 1 {
		t.Fatalf("expected 1 problem MR from project-b, got %d", len(report.ProblemMRs))
	}
}

func TestRun_MRErrorDoesNotAbortRepo(t *testing.T) {
	now := time.Now()
	client := &mockClient{
		mrs: map[string][]gitlab.MergeRequest{
			"group/project": {
				{IID: 1, Title: "mr with approval error", CreatedAt: now.Add(-48 * time.Hour)},
				{IID: 2, Title: "mr ok", Author: gitlab.User{Username: "ivanov"}, CreatedAt: now.Add(-48 * time.Hour)},
			},
		},
		approvalsErr: map[string]error{
			key("group/project", 1): errors.New("timeout"),
		},
	}

	report := Run(context.Background(), client, []string{"group/project"}, 2, 24, now)

	if len(report.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(report.Errors))
	}
	if len(report.ProblemMRs) != 1 {
		t.Fatalf("expected 1 problem MR (mr 2), got %d", len(report.ProblemMRs))
	}
	if report.ProblemMRs[0].IID != 2 {
		t.Errorf("expected problem MR iid=2, got %d", report.ProblemMRs[0].IID)
	}
}

func TestRun_NoProblemsNoErrors(t *testing.T) {
	now := time.Now()
	client := &mockClient{
		mrs: map[string][]gitlab.MergeRequest{
			"group/project": {},
		},
	}

	report := Run(context.Background(), client, []string{"group/project"}, 2, 24, now)

	if len(report.ProblemMRs) != 0 || len(report.Errors) != 0 {
		t.Fatalf("expected empty report, got %+v", report)
	}
}
