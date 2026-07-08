package checker

import (
	"fmt"
	"strings"
	"time"
)

const timeLayout = "2006-01-02 15:04"

// FormatRootMessage формирует текст корневого сообщения со списком проблемных МР
// в формате, описанном в разделе 6.1 ТЗ.
func FormatRootMessage(problems []ProblemMR, checkedAt time.Time) string {
	var b strings.Builder

	fmt.Fprintf(&b, "⚠️ **МР без достаточного количества ревью** (проверка от %s)\n\n", checkedAt.Format(timeLayout))

	for _, p := range problems {
		fmt.Fprintf(&b, "**%s** !%d — [%s](%s)\n", p.Repository, p.IID, p.Title, p.WebURL)
		fmt.Fprintf(&b, "Автор: @%s · создан %s · ревьюеров: %d/%d%s\n\n",
			p.Author, humanizeAge(checkedAt, p.CreatedAt), len(p.Reviewers), p.MinReviewers, formatReviewers(p.Reviewers))
	}

	fmt.Fprintf(&b, "Всего найдено: %d", len(problems))

	return b.String()
}

// FormatErrorsMessage формирует текст сообщения об ошибках проверки
// в формате, описанном в разделе 6.2 ТЗ.
func FormatErrorsMessage(errs []CheckError) string {
	var b strings.Builder

	b.WriteString("❌ Ошибки при проверке репозиториев:\n\n")

	lines := make([]string, 0, len(errs))
	for _, e := range errs {
		lines = append(lines, fmt.Sprintf("• %s: %s", e.Repository, e.Message))
	}
	b.WriteString(strings.Join(lines, "\n"))

	return b.String()
}

func formatReviewers(reviewers []string) string {
	if len(reviewers) == 0 {
		return ""
	}

	mentions := make([]string, 0, len(reviewers))
	for _, r := range reviewers {
		mentions = append(mentions, "@"+r)
	}

	return fmt.Sprintf(" (%s)", strings.Join(mentions, ", "))
}

// humanizeAge возвращает возраст МР в человекочитаемом виде на русском языке:
// в часах, если прошло меньше 48 часов, иначе в днях.
func humanizeAge(now, createdAt time.Time) string {
	hours := int(now.Sub(createdAt).Hours())

	if hours < 48 {
		return fmt.Sprintf("%d %s назад", hours, pluralRu(hours, "час", "часа", "часов"))
	}

	days := hours / 24
	return fmt.Sprintf("%d %s назад", days, pluralRu(days, "день", "дня", "дней"))
}

// pluralRu выбирает правильную форму русского слова по числу n.
func pluralRu(n int, one, few, many string) string {
	if n < 0 {
		n = -n
	}

	if n%100 >= 11 && n%100 <= 14 {
		return many
	}

	switch n % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	default:
		return many
	}
}
