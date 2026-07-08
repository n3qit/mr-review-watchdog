package checker

import (
	"strings"
	"testing"
	"time"
)

func TestFormatRootMessage(t *testing.T) {
	checkedAt := time.Date(2026, 7, 8, 9, 0, 0, 0, time.UTC)

	problems := []ProblemMR{
		{
			Repository:   "group/project-a",
			IID:          123,
			Title:        "Добавить кэширование в API",
			WebURL:       "https://gitlab.example.com/group/project-a/-/merge_requests/123",
			Author:       "ivanov",
			CreatedAt:    checkedAt.Add(-48 * time.Hour),
			Reviewers:    []string{"petrov"},
			MinReviewers: 2,
		},
		{
			Repository:   "group/project-b",
			IID:          45,
			Title:        "Fix flaky test in auth module",
			WebURL:       "https://gitlab.example.com/group/project-b/-/merge_requests/45",
			Author:       "sidorov",
			CreatedAt:    checkedAt.Add(-36 * time.Hour),
			Reviewers:    nil,
			MinReviewers: 2,
		},
	}

	msg := FormatRootMessage(problems, checkedAt)

	wantContains := []string{
		"⚠️ **МР без достаточного количества ревью** (проверка от 2026-07-08 09:00)",
		"**group/project-a** !123 — [Добавить кэширование в API](https://gitlab.example.com/group/project-a/-/merge_requests/123)",
		"Автор: @ivanov · создан 2 дня назад · ревьюеров: 1/2 (@petrov)",
		"**group/project-b** !45 — [Fix flaky test in auth module](https://gitlab.example.com/group/project-b/-/merge_requests/45)",
		"Автор: @sidorov · создан 36 часов назад · ревьюеров: 0/2",
		"Всего найдено: 2",
	}

	for _, want := range wantContains {
		if !strings.Contains(msg, want) {
			t.Errorf("сообщение не содержит %q\nполное сообщение:\n%s", want, msg)
		}
	}

	if strings.Contains(msg, "0/2 (") {
		t.Errorf("сообщение не должно содержать список ревьюеров при их отсутствии:\n%s", msg)
	}
}

func TestFormatErrorsMessage(t *testing.T) {
	iid78 := 78
	errs := []CheckError{
		{Repository: "group/project-c", Message: "не удалось получить список МР (GitLab API вернул 502)"},
		{Repository: "group/project-d", MRIID: &iid78, Message: "превышен таймаут запроса approvals для !78"},
	}

	msg := FormatErrorsMessage(errs)

	wantContains := []string{
		"❌ Ошибки при проверке репозиториев:",
		"• group/project-c: не удалось получить список МР (GitLab API вернул 502)",
		"• group/project-d: превышен таймаут запроса approvals для !78",
	}

	for _, want := range wantContains {
		if !strings.Contains(msg, want) {
			t.Errorf("сообщение не содержит %q\nполное сообщение:\n%s", want, msg)
		}
	}
}

func TestPluralRu(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "час"}, {2, "часа"}, {5, "часов"},
		{11, "часов"}, {21, "час"}, {22, "часа"}, {25, "часов"},
	}

	for _, c := range cases {
		got := pluralRu(c.n, "час", "часа", "часов")
		if got != c.want {
			t.Errorf("pluralRu(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
