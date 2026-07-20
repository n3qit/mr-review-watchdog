// Команда mr-review-watchdog проверяет заданный список репозиториев GitLab на наличие
// merge request'ов без достаточного количества ревью и публикует отчёт в Mattermost.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/n3qit/mr-review-watchdog/internal/calendar"
	"github.com/n3qit/mr-review-watchdog/internal/checker"
	"github.com/n3qit/mr-review-watchdog/internal/config"
	"github.com/n3qit/mr-review-watchdog/internal/gitlab"
	"github.com/n3qit/mr-review-watchdog/internal/mattermost"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	configPath := flag.String("config", "", "путь к конфигурационному YAML-файлу")
	flag.Parse()

	if *configPath == "" {
		slog.Error("не указан путь к конфигурационному файлу (флаг --config)")
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("невалидная конфигурация", "error", err)
		os.Exit(1)
	}

	gitlabClient := gitlab.NewClient(cfg.GitLab.BaseURL, cfg.GitLab.Token)
	mmClient := mattermost.NewClient(cfg.Mattermost.BaseURL, cfg.Mattermost.Token)
	calendarClient := calendar.NewClient(cfg.Calendar.BaseURL)

	if err := run(context.Background(), gitlabClient, calendarClient, mmClient, cfg, time.Now()); err != nil {
		slog.Error("проверка завершилась с ошибкой", "error", err)
		os.Exit(1)
	}
}

// mattermostClient — минимальный интерфейс Mattermost-клиента, используемый оркестрацией.
// Позволяет подменять реализацию моками в тестах.
type mattermostClient interface {
	CreatePost(ctx context.Context, channelID, message string) (string, error)
	CreateReply(ctx context.Context, channelID, rootID, message string) (string, error)
}

// run связывает компоненты: обходит репозитории, строит отчёт и отправляет
// сообщения в Mattermost согласно правилам раздела "Уведомления в Mattermost" README.
func run(ctx context.Context, gitlabClient checker.GitLabClient, calendarClient checker.CalendarClient, mmClient mattermostClient, cfg *config.Config, now time.Time) error {
	slog.Info("запуск проверки", "repositories", len(cfg.Repositories),
		"min_reviewers", cfg.Check.MinReviewers, "min_age_hours", cfg.Check.MinAgeHours,
		"team_group", cfg.GitLab.TeamGroup)

	report, err := checker.Run(ctx, gitlabClient, calendarClient, cfg.Repositories, checker.Options{
		MinReviewers: cfg.Check.MinReviewers,
		MinAgeHours:  cfg.Check.MinAgeHours,
		TeamGroup:    cfg.GitLab.TeamGroup,
		Now:          now,
	})
	if err != nil {
		return err
	}

	slog.Info("проверка завершена",
		"repositories", len(cfg.Repositories),
		"mrs_checked", report.TotalChecked,
		"young_found", len(report.YoungMRs),
		"problems_found", len(report.ProblemMRs),
		"errors", len(report.Errors),
	)

	if len(report.ProblemMRs) == 0 && len(report.YoungMRs) == 0 && len(report.Errors) == 0 {
		slog.Info("молодых, проблемных МР и ошибок не найдено, сообщения не отправляются")
		return nil
	}

	var rootPostID string

	if len(report.ProblemMRs) > 0 || len(report.YoungMRs) > 0 {
		rootMessage := checker.FormatRootMessage(report.YoungMRs, report.ProblemMRs, now)

		postID, err := mmClient.CreatePost(ctx, cfg.Mattermost.ChannelID, rootMessage)
		if err != nil {
			return err
		}
		rootPostID = postID

		slog.Info("отправлено корневое сообщение", "young", len(report.YoungMRs), "problems", len(report.ProblemMRs), "post_id", rootPostID)
	}

	if len(report.Errors) > 0 {
		errorsMessage := checker.FormatErrorsMessage(report.Errors)

		var err error
		if rootPostID != "" {
			_, err = mmClient.CreateReply(ctx, cfg.Mattermost.ChannelID, rootPostID, errorsMessage)
		} else {
			_, err = mmClient.CreatePost(ctx, cfg.Mattermost.ChannelID, errorsMessage)
		}
		if err != nil {
			return err
		}

		slog.Info("отправлено сообщение об ошибках", "count", len(report.Errors), "threaded", rootPostID != "")
	}

	return nil
}
