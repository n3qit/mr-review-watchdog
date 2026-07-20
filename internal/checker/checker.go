// Package checker содержит бизнес-логику проверки МР: фильтрацию
// draft/возраста/команды, подсчёт уникальных ревьюеров, определение статуса
// и сборку отчёта об ошибках.
package checker

import (
	"context"
	"fmt"
	"time"

	"github.com/n3qit/mr-review-watchdog/internal/gitlab"
)

// GitLabClient — минимальный интерфейс GitLab-клиента, используемый checker'ом.
// Позволяет подменять реализацию моками в тестах.
type GitLabClient interface {
	ListOpenMergeRequests(ctx context.Context, projectPath string) ([]gitlab.MergeRequest, error)
	GetApprovals(ctx context.Context, projectPath string, mrIID int) ([]string, error)
	ListDiscussions(ctx context.Context, projectPath string, mrIID int) ([]gitlab.Discussion, error)
	ListGroupMembers(ctx context.Context, groupPath string) ([]string, error)
}

// Status — статус МР в процессе ревью.
type Status string

const (
	StatusAwaitingReview  Status = "ожидает ревью"
	StatusAwaitingChanges Status = "ожидает исправления"
	StatusFixed           Status = "исправлено"
)

// ProblemMR — МР, не набравший минимально необходимое количество ревьюеров.
type ProblemMR struct {
	Repository      string
	IID             int
	Title           string
	WebURL          string
	Author          string
	CreatedAt       time.Time
	Reviewers       []string
	MinReviewers    int
	Status          Status
	ApprovalsCount  int
	ThreadsTotal    int
	ThreadsResolved int
}

// CheckError — ошибка, возникшая при обработке репозитория или конкретного МР.
type CheckError struct {
	Repository string
	MRIID      *int
	Message    string
}

// Report — результат проверки списка репозиториев.
type Report struct {
	ProblemMRs []ProblemMR
	Errors     []CheckError
	// TotalChecked — количество не-draft МР от участников команды, прошедших
	// фильтр по возрасту и оценённых на количество ревьюеров (независимо от результата).
	TotalChecked int
}

// Options — параметры проверки.
type Options struct {
	MinReviewers int
	MinAgeHours  int
	// TeamGroup — путь до группы GitLab, чьи прямые участники считаются
	// "нашей командой". Если пусто, фильтрация по команде не применяется.
	TeamGroup string
	Now       time.Time
}

// Run проверяет список репозиториев и возвращает найденные проблемные МР и ошибки.
// Ошибка при обработке одного репозитория или МР не прерывает проверку остальных.
// Ошибка получения списка участников команды фатальна для всего прогона, так как
// от неё зависит корректность фильтрации по всем репозиториям.
func Run(ctx context.Context, client GitLabClient, calendarClient CalendarClient, repositories []string, opts Options) (Report, error) {
	var report Report

	teamMembers, err := loadTeamMembers(ctx, client, opts.TeamGroup)
	if err != nil {
		return Report{}, err
	}

	holidays := newHolidayCache(calendarClient)

	for _, repo := range repositories {
		mrs, err := client.ListOpenMergeRequests(ctx, repo)
		if err != nil {
			report.Errors = append(report.Errors, CheckError{
				Repository: repo,
				Message:    fmt.Sprintf("не удалось получить список МР (%s)", err),
			})
			continue
		}

		for _, mr := range mrs {
			if mr.IsDraft() {
				continue
			}
			if teamMembers != nil {
				if _, ok := teamMembers[mr.Author.Username]; !ok {
					continue
				}
			}

			iid := mr.IID

			elapsed, err := businessHoursElapsed(ctx, holidays, mr.CreatedAt, opts.Now)
			if err != nil {
				report.Errors = append(report.Errors, CheckError{
					Repository: repo,
					MRIID:      &iid,
					Message:    fmt.Sprintf("определение рабочего возраста !%d: %s", mr.IID, err),
				})
				continue
			}
			if elapsed < time.Duration(opts.MinAgeHours)*time.Hour {
				continue
			}

			report.TotalChecked++

			stats, err := reviewStatsOf(ctx, client, repo, mr)
			if err != nil {
				report.Errors = append(report.Errors, CheckError{
					Repository: repo,
					MRIID:      &iid,
					Message:    err.Error(),
				})
				continue
			}

			if len(stats.reviewers) < opts.MinReviewers {
				report.ProblemMRs = append(report.ProblemMRs, ProblemMR{
					Repository:      repo,
					IID:             mr.IID,
					Title:           mr.Title,
					WebURL:          mr.WebURL,
					Author:          mr.Author.Username,
					CreatedAt:       mr.CreatedAt,
					Reviewers:       stats.reviewers,
					MinReviewers:    opts.MinReviewers,
					Status:          deriveStatus(stats.approvalsCount, stats.threadsTotal, stats.threadsResolved),
					ApprovalsCount:  stats.approvalsCount,
					ThreadsTotal:    stats.threadsTotal,
					ThreadsResolved: stats.threadsResolved,
				})
			}
		}
	}

	return report, nil
}

// loadTeamMembers возвращает набор имён пользователей — прямых участников группы.
// Если groupPath пуст, возвращает nil (фильтрация по команде отключена).
func loadTeamMembers(ctx context.Context, client GitLabClient, groupPath string) (map[string]struct{}, error) {
	if groupPath == "" {
		return nil, nil
	}

	members, err := client.ListGroupMembers(ctx, groupPath)
	if err != nil {
		return nil, fmt.Errorf("получение участников группы %q: %w", groupPath, err)
	}

	set := make(map[string]struct{}, len(members))
	for _, username := range members {
		set[username] = struct{}{}
	}

	return set, nil
}

// deriveStatus вычисляет статус МР по статистике ревью.
func deriveStatus(approvalsCount, threadsTotal, threadsResolved int) Status {
	if threadsResolved < threadsTotal {
		return StatusAwaitingChanges
	}
	if approvalsCount > 0 || threadsTotal > 0 {
		return StatusFixed
	}
	return StatusAwaitingReview
}

type reviewStats struct {
	reviewers       []string
	approvalsCount  int
	threadsTotal    int
	threadsResolved int
}

// reviewStatsOf собирает статистику ревью МР: уникальных ревьюеров (объединение
// approvals и авторов non-system заметок, исключая автора самого МР), количество
// аппрувов и статистику по резолвящимся тредам.
func reviewStatsOf(ctx context.Context, client GitLabClient, repo string, mr gitlab.MergeRequest) (reviewStats, error) {
	approvals, err := client.GetApprovals(ctx, repo, mr.IID)
	if err != nil {
		return reviewStats{}, fmt.Errorf("получение approvals для !%d: %w", mr.IID, err)
	}

	discussions, err := client.ListDiscussions(ctx, repo, mr.IID)
	if err != nil {
		return reviewStats{}, fmt.Errorf("получение discussions для !%d: %w", mr.IID, err)
	}

	seen := make(map[string]struct{})
	var reviewers []string

	addReviewer := func(username string) {
		if username == "" || username == mr.Author.Username {
			return
		}
		if _, ok := seen[username]; ok {
			return
		}
		seen[username] = struct{}{}
		reviewers = append(reviewers, username)
	}

	for _, username := range approvals {
		addReviewer(username)
	}

	var threadsTotal, threadsResolved int
	for _, d := range discussions {
		if isResolvableThread(d) {
			threadsTotal++
			if isThreadResolved(d) {
				threadsResolved++
			}
		}

		for _, note := range d.Notes {
			if note.System {
				continue
			}
			addReviewer(note.Author.Username)
		}
	}

	return reviewStats{
		reviewers:       reviewers,
		approvalsCount:  len(approvals),
		threadsTotal:    threadsTotal,
		threadsResolved: threadsResolved,
	}, nil
}

// isResolvableThread сообщает, содержит ли тред хотя бы одну резолвящуюся заметку
// (то есть является полноценным code-review тредом, а не просто общим комментарием).
func isResolvableThread(d gitlab.Discussion) bool {
	for _, n := range d.Notes {
		if n.Resolvable {
			return true
		}
	}
	return false
}

// isThreadResolved сообщает, резолвнуты ли все резолвящиеся заметки треда.
func isThreadResolved(d gitlab.Discussion) bool {
	for _, n := range d.Notes {
		if n.Resolvable && !n.Resolved {
			return false
		}
	}
	return true
}
