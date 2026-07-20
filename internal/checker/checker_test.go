package checker

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/n3qit/mr-review-watchdog/internal/gitlab"
)

// testNow — фиксированный четверг (2026-07-16), чтобы смещения вроде
// "-48h"/"-1h" в тестах не зависели от дня недели, в который реально
// запускаются тесты, и не пересекали выходные непреднамеренно.
func testNow() time.Time {
	return time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
}

// noHolidays — календарь без праздников: на возраст МР влияют только
// выходные дни (Сб/Вс), что и используется в тестах, не посвящённых
// собственно календарной логике (она отдельно покрыта в businesstime_test.go).
func noHolidays() *stubCalendarClient {
	return &stubCalendarClient{}
}

type mockClient struct {
	mrs             map[string][]gitlab.MergeRequest
	mrsErr          map[string]error
	approvals       map[string][]string
	approvalsErr    map[string]error
	discussions     map[string][]gitlab.Discussion
	discussionsErr  map[string]error
	groupMembers    map[string][]string
	groupMembersErr error
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

func (m *mockClient) ListDiscussions(ctx context.Context, projectPath string, mrIID int) ([]gitlab.Discussion, error) {
	k := key(projectPath, mrIID)
	if err, ok := m.discussionsErr[k]; ok {
		return nil, err
	}
	return m.discussions[k], nil
}

func (m *mockClient) ListGroupMembers(ctx context.Context, groupPath string) ([]string, error) {
	if m.groupMembersErr != nil {
		return nil, m.groupMembersErr
	}
	return m.groupMembers[groupPath], nil
}

func TestRun_ExcludesDraftMRs(t *testing.T) {
	now := testNow()
	client := &mockClient{
		mrs: map[string][]gitlab.MergeRequest{
			"group/project": {
				{IID: 1, Title: "draft mr", Draft: true, CreatedAt: now.Add(-48 * time.Hour)},
				{IID: 2, Title: "legacy wip", WorkInProgress: true, CreatedAt: now.Add(-48 * time.Hour)},
			},
		},
	}

	report, err := Run(context.Background(), client, noHolidays(), []string{"group/project"}, Options{MinReviewers: 2, MinAgeHours: 24, Now: now})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(report.ProblemMRs) != 0 {
		t.Errorf("expected 0 problem MRs, got %d", len(report.ProblemMRs))
	}
}

func TestRun_ExcludesYoungMRs(t *testing.T) {
	now := testNow()
	client := &mockClient{
		mrs: map[string][]gitlab.MergeRequest{
			"group/project": {
				{IID: 1, Title: "fresh mr", CreatedAt: now.Add(-1 * time.Hour)},
			},
		},
	}

	report, err := Run(context.Background(), client, noHolidays(), []string{"group/project"}, Options{MinReviewers: 2, MinAgeHours: 24, Now: now})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(report.ProblemMRs) != 0 {
		t.Errorf("expected 0 problem MRs, got %d", len(report.ProblemMRs))
	}
}

func TestRun_CountsUniqueReviewersFromApprovalsAndDiscussions(t *testing.T) {
	now := testNow()
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
		discussions: map[string][]gitlab.Discussion{
			key("group/project", 1): {
				{Notes: []gitlab.Note{{Author: gitlab.User{Username: "petrov"}, System: false}}},  // дубликат с approvals
				{Notes: []gitlab.Note{{Author: gitlab.User{Username: "sidorov"}, System: false}}}, // новый ревьюер
				{Notes: []gitlab.Note{{Author: gitlab.User{Username: "ivanov"}, System: false}}},  // автор, исключается
				{Notes: []gitlab.Note{{Author: gitlab.User{Username: "bot"}, System: true}}},      // системная заметка, исключается
			},
		},
	}

	report, err := Run(context.Background(), client, noHolidays(), []string{"group/project"}, Options{MinReviewers: 2, MinAgeHours: 24, Now: now})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(report.ProblemMRs) != 0 {
		t.Fatalf("expected MR to have 2 reviewers (not problem), got problems: %+v", report.ProblemMRs)
	}
}

func TestRun_ProblemMRWhenNotEnoughReviewers(t *testing.T) {
	now := testNow()
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

	report, err := Run(context.Background(), client, noHolidays(), []string{"group/project"}, Options{MinReviewers: 2, MinAgeHours: 24, Now: now})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(report.ProblemMRs) != 1 {
		t.Fatalf("expected 1 problem MR, got %d", len(report.ProblemMRs))
	}
	if len(report.ProblemMRs[0].Reviewers) != 1 {
		t.Errorf("expected 1 reviewer, got %d", len(report.ProblemMRs[0].Reviewers))
	}
	if report.ProblemMRs[0].ApprovalsCount != 1 {
		t.Errorf("expected ApprovalsCount 1, got %d", report.ProblemMRs[0].ApprovalsCount)
	}
}

func TestRun_RepoErrorDoesNotAbortOthers(t *testing.T) {
	now := testNow()
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

	report, err := Run(context.Background(), client, noHolidays(), []string{"group/project-a", "group/project-b"}, Options{MinReviewers: 2, MinAgeHours: 24, Now: now})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(report.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %+v", len(report.Errors), report.Errors)
	}
	if len(report.ProblemMRs) != 1 {
		t.Fatalf("expected 1 problem MR from project-b, got %d", len(report.ProblemMRs))
	}
}

func TestRun_MRErrorDoesNotAbortRepo(t *testing.T) {
	now := testNow()
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

	report, err := Run(context.Background(), client, noHolidays(), []string{"group/project"}, Options{MinReviewers: 2, MinAgeHours: 24, Now: now})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

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
	now := testNow()
	client := &mockClient{
		mrs: map[string][]gitlab.MergeRequest{
			"group/project": {},
		},
	}

	report, err := Run(context.Background(), client, noHolidays(), []string{"group/project"}, Options{MinReviewers: 2, MinAgeHours: 24, Now: now})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(report.ProblemMRs) != 0 || len(report.Errors) != 0 {
		t.Fatalf("expected empty report, got %+v", report)
	}
}

func TestRun_TeamFilter_ExcludesNonTeamAuthors(t *testing.T) {
	now := testNow()
	client := &mockClient{
		mrs: map[string][]gitlab.MergeRequest{
			"group/project": {
				{IID: 1, Title: "outsider mr", Author: gitlab.User{Username: "outsider"}, CreatedAt: now.Add(-48 * time.Hour)},
				{IID: 2, Title: "team mr", Author: gitlab.User{Username: "ivanov"}, CreatedAt: now.Add(-48 * time.Hour)},
			},
		},
		groupMembers: map[string][]string{
			"group/our-team": {"ivanov", "petrov"},
		},
	}

	report, err := Run(context.Background(), client, noHolidays(), []string{"group/project"}, Options{MinReviewers: 2, MinAgeHours: 24, TeamGroup: "group/our-team", Now: now})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if report.TotalChecked != 1 {
		t.Fatalf("expected 1 checked MR (outsider excluded), got %d", report.TotalChecked)
	}
	if len(report.ProblemMRs) != 1 || report.ProblemMRs[0].IID != 2 {
		t.Fatalf("expected only team MR (iid=2) to be a problem, got %+v", report.ProblemMRs)
	}
}

func TestRun_TeamFilter_EmptyGroupMeansNoFilter(t *testing.T) {
	now := testNow()
	client := &mockClient{
		mrs: map[string][]gitlab.MergeRequest{
			"group/project": {
				{IID: 1, Title: "anyone's mr", Author: gitlab.User{Username: "outsider"}, CreatedAt: now.Add(-48 * time.Hour)},
			},
		},
	}

	report, err := Run(context.Background(), client, noHolidays(), []string{"group/project"}, Options{MinReviewers: 2, MinAgeHours: 24, Now: now})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if report.TotalChecked != 1 {
		t.Fatalf("expected 1 checked MR without team filter, got %d", report.TotalChecked)
	}
}

func TestRun_TeamFilter_FetchErrorIsFatal(t *testing.T) {
	now := testNow()
	client := &mockClient{
		groupMembersErr: errors.New("502 bad gateway"),
	}

	_, err := Run(context.Background(), client, noHolidays(), []string{"group/project"}, Options{MinReviewers: 2, MinAgeHours: 24, TeamGroup: "group/our-team", Now: now})
	if err == nil {
		t.Fatal("expected fatal error when team member fetch fails, got nil")
	}
}

func TestRun_ThreadStats(t *testing.T) {
	now := testNow()
	client := &mockClient{
		mrs: map[string][]gitlab.MergeRequest{
			"group/project": {
				{IID: 1, Title: "mr with threads", Author: gitlab.User{Username: "ivanov"}, CreatedAt: now.Add(-48 * time.Hour)},
			},
		},
		discussions: map[string][]gitlab.Discussion{
			key("group/project", 1): {
				// резолвнутый code-review тред
				{Notes: []gitlab.Note{{Author: gitlab.User{Username: "petrov"}, Resolvable: true, Resolved: true}}},
				// нерезолвнутый code-review тред
				{Notes: []gitlab.Note{{Author: gitlab.User{Username: "petrov"}, Resolvable: true, Resolved: false}}},
				// обычный комментарий, не тред
				{Notes: []gitlab.Note{{Author: gitlab.User{Username: "sidorov"}, Resolvable: false}}},
			},
		},
	}

	report, err := Run(context.Background(), client, noHolidays(), []string{"group/project"}, Options{MinReviewers: 5, MinAgeHours: 24, Now: now})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(report.ProblemMRs) != 1 {
		t.Fatalf("expected 1 problem MR, got %d", len(report.ProblemMRs))
	}

	mr := report.ProblemMRs[0]
	if mr.ThreadsTotal != 2 {
		t.Errorf("ThreadsTotal = %d, want 2", mr.ThreadsTotal)
	}
	if mr.ThreadsResolved != 1 {
		t.Errorf("ThreadsResolved = %d, want 1", mr.ThreadsResolved)
	}
	if mr.Status != StatusAwaitingChanges {
		t.Errorf("Status = %q, want %q", mr.Status, StatusAwaitingChanges)
	}
}

func TestRun_CalendarErrorIsPerMR(t *testing.T) {
	now := testNow()
	client := &mockClient{
		mrs: map[string][]gitlab.MergeRequest{
			"group/project-b": {
				{IID: 1, Title: "needs review", Author: gitlab.User{Username: "ivanov"}, CreatedAt: now.Add(-48 * time.Hour)},
			},
		},
	}

	report, err := Run(context.Background(), client, &stubCalendarClient{err: errors.New("502 bad gateway")}, []string{"group/project-b"}, Options{MinReviewers: 2, MinAgeHours: 24, Now: now})
	if err != nil {
		t.Fatalf("Run returned error: %v, want nil (calendar errors are per-MR)", err)
	}

	if len(report.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %+v", len(report.Errors), report.Errors)
	}
	if len(report.ProblemMRs) != 0 {
		t.Fatalf("expected 0 problem MRs (age undetermined), got %d", len(report.ProblemMRs))
	}
	if report.TotalChecked != 0 {
		t.Fatalf("expected TotalChecked=0 (age undetermined MR not counted), got %d", report.TotalChecked)
	}
}

func TestDeriveStatus(t *testing.T) {
	cases := []struct {
		name            string
		approvalsCount  int
		threadsTotal    int
		threadsResolved int
		want            Status
	}{
		{"нет активности", 0, 0, 0, StatusAwaitingReview},
		{"есть нерезолвнутый тред", 0, 2, 1, StatusAwaitingChanges},
		{"все треды резолвнуты, аппрувов нет", 0, 2, 2, StatusFixed},
		{"есть аппрув, тредов нет", 1, 0, 0, StatusFixed},
		{"есть аппрув и нерезолвнутый тред", 1, 1, 0, StatusAwaitingChanges},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := deriveStatus(c.approvalsCount, c.threadsTotal, c.threadsResolved)
			if got != c.want {
				t.Errorf("deriveStatus(%d, %d, %d) = %q, want %q", c.approvalsCount, c.threadsTotal, c.threadsResolved, got, c.want)
			}
		})
	}
}
