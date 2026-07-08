// Package checker содержит бизнес-логику проверки МР: фильтрацию
// draft/возраста, подсчёт уникальных ревьюеров и сборку отчёта об ошибках.
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
	ListNotes(ctx context.Context, projectPath string, mrIID int) ([]gitlab.Note, error)
}

// ProblemMR — МР, не набравший минимально необходимое количество ревьюеров.
type ProblemMR struct {
	Repository   string
	IID          int
	Title        string
	WebURL       string
	Author       string
	CreatedAt    time.Time
	Reviewers    []string
	MinReviewers int
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
	// TotalChecked — количество не-draft МР, прошедших фильтр по возрасту
	// и оценённых на количество ревьюеров (независимо от результата).
	TotalChecked int
}

// Run проверяет список репозиториев и возвращает найденные проблемные МР и ошибки.
// Ошибка при обработке одного репозитория или МР не прерывает проверку остальных.
func Run(ctx context.Context, client GitLabClient, repositories []string, minReviewers, minAgeHours int, now time.Time) Report {
	var report Report

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
			if now.Sub(mr.CreatedAt) < time.Duration(minAgeHours)*time.Hour {
				continue
			}

			report.TotalChecked++

			iid := mr.IID
			reviewers, err := reviewersOf(ctx, client, repo, mr)
			if err != nil {
				report.Errors = append(report.Errors, CheckError{
					Repository: repo,
					MRIID:      &iid,
					Message:    err.Error(),
				})
				continue
			}

			if len(reviewers) < minReviewers {
				report.ProblemMRs = append(report.ProblemMRs, ProblemMR{
					Repository:   repo,
					IID:          mr.IID,
					Title:        mr.Title,
					WebURL:       mr.WebURL,
					Author:       mr.Author.Username,
					CreatedAt:    mr.CreatedAt,
					Reviewers:    reviewers,
					MinReviewers: minReviewers,
				})
			}
		}
	}

	return report
}

// reviewersOf возвращает уникальных ревьюеров МР: объединение пользователей
// из approvals и non-system notes, исключая автора самого МР.
func reviewersOf(ctx context.Context, client GitLabClient, repo string, mr gitlab.MergeRequest) ([]string, error) {
	approvals, err := client.GetApprovals(ctx, repo, mr.IID)
	if err != nil {
		return nil, fmt.Errorf("получение approvals для !%d: %w", mr.IID, err)
	}

	notes, err := client.ListNotes(ctx, repo, mr.IID)
	if err != nil {
		return nil, fmt.Errorf("получение notes для !%d: %w", mr.IID, err)
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
	for _, note := range notes {
		if note.System {
			continue
		}
		addReviewer(note.Author.Username)
	}

	return reviewers, nil
}
