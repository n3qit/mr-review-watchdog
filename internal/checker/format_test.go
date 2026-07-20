package checker

import (
	"strings"
	"testing"
	"time"
)

func sampleProblems(checkedAt time.Time) []MRSummary {
	return []MRSummary{
		{
			Repository:      "group/project-a",
			IID:             123,
			Title:           "Добавить кэширование в API",
			WebURL:          "https://gitlab.example.com/group/project-a/-/merge_requests/123",
			Author:          "ivanov",
			CreatedAt:       checkedAt.Add(-48 * time.Hour),
			Reviewers:       []string{"petrov"},
			MinReviewers:    2,
			Status:          StatusAwaitingChanges,
			ApprovalsCount:  0,
			ThreadsTotal:    3,
			ThreadsResolved: 1,
		},
		{
			Repository:      "group/project-b",
			IID:             45,
			Title:           "Fix flaky test in auth module",
			WebURL:          "https://gitlab.example.com/group/project-b/-/merge_requests/45",
			Author:          "sidorov",
			CreatedAt:       checkedAt.Add(-36 * time.Hour),
			Reviewers:       nil,
			MinReviewers:    2,
			Status:          StatusAwaitingReview,
			ApprovalsCount:  0,
			ThreadsTotal:    0,
			ThreadsResolved: 0,
		},
	}
}

func sampleYoung(checkedAt time.Time) []MRSummary {
	return []MRSummary{
		{
			Repository:      "group/project-c",
			IID:             88,
			Title:           "Fix typo",
			WebURL:          "https://gitlab.example.com/group/project-c/-/merge_requests/88",
			Author:          "sidorov",
			CreatedAt:       checkedAt.Add(-2 * time.Hour),
			Reviewers:       nil,
			MinReviewers:    2,
			Status:          StatusAwaitingReview,
			ApprovalsCount:  0,
			ThreadsTotal:    0,
			ThreadsResolved: 0,
		},
	}
}

func TestFormatRootMessage_ProblemsOnly(t *testing.T) {
	checkedAt := time.Date(2026, 7, 8, 9, 0, 0, 0, time.UTC)

	msg := FormatRootMessage(nil, sampleProblems(checkedAt), checkedAt)

	wantContains := []string{
		"⚠️ **МР без достаточного количества ревью**",
		"- **group/project-a** !123 — [Добавить кэширование в API](https://gitlab.example.com/group/project-a/-/merge_requests/123)",
		"Автор: @ivanov · создан 2 дня назад · статус: ожидает исправления",
		"ревьюеров: 1/2 (@petrov) · аппрувов: 0 · тредов: 3 (исправлено 1/3)",
		"- **group/project-b** !45 — [Fix flaky test in auth module](https://gitlab.example.com/group/project-b/-/merge_requests/45)",
		"Автор: @sidorov · создан 36 часов назад · статус: ожидает ревью",
		"ревьюеров: 0/2 · аппрувов: 0 · тредов: 0 (исправлено 0/0)",
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
	if strings.Contains(msg, "проверка от") {
		t.Errorf("заголовок раздела не должен содержать метку времени:\n%s", msg)
	}
	if strings.Contains(msg, "🕒") {
		t.Errorf("сообщение без молодых МР не должно содержать раздел молодых:\n%s", msg)
	}
}

func TestFormatRootMessage_YoungOnly(t *testing.T) {
	checkedAt := time.Date(2026, 7, 8, 9, 0, 0, 0, time.UTC)

	msg := FormatRootMessage(sampleYoung(checkedAt), nil, checkedAt)

	wantContains := []string{
		"🕒 **МР, которые скоро попадут в проверку**",
		"- **group/project-c** !88 — [Fix typo](https://gitlab.example.com/group/project-c/-/merge_requests/88)",
		"Автор: @sidorov · создан 2 часа назад · статус: ожидает ревью",
		"ревьюеров: 0/2 · аппрувов: 0 · тредов: 0 (исправлено 0/0)",
		"Скоро попадёт в проверку: 1",
	}

	for _, want := range wantContains {
		if !strings.Contains(msg, want) {
			t.Errorf("сообщение не содержит %q\nполное сообщение:\n%s", want, msg)
		}
	}

	if strings.Contains(msg, "⚠️") || strings.Contains(msg, "Всего найдено") {
		t.Errorf("сообщение без проблемных МР не должно содержать раздел проблемных:\n%s", msg)
	}
}

func TestFormatRootMessage_BothSections(t *testing.T) {
	checkedAt := time.Date(2026, 7, 8, 9, 0, 0, 0, time.UTC)

	msg := FormatRootMessage(sampleYoung(checkedAt), sampleProblems(checkedAt), checkedAt)

	youngIdx := strings.Index(msg, "🕒")
	problemsIdx := strings.Index(msg, "⚠️")
	if youngIdx == -1 || problemsIdx == -1 || youngIdx > problemsIdx {
		t.Errorf("раздел молодых МР должен идти раньше раздела проблемных:\n%s", msg)
	}
}

func TestFormatRootMessage_Empty(t *testing.T) {
	checkedAt := time.Date(2026, 7, 8, 9, 0, 0, 0, time.UTC)

	msg := FormatRootMessage(nil, nil, checkedAt)
	if msg != "" {
		t.Errorf("ожидалась пустая строка при отсутствии молодых и проблемных МР, получено:\n%s", msg)
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
