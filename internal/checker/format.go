package checker

import (
	"fmt"
	"strings"
	"time"
)

const timeLayout = "2006-01-02 15:04"

// FormatRootMessage формирует текст корневого сообщения. Раздел "молодых" МР
// (ещё не достигших порога возраста) идёт первым, раздел проблемных МР —
// вторым; каждый раздел рендерится только если в нём есть записи.
func FormatRootMessage(young, problems []MRSummary, checkedAt time.Time) string {
	sections := make([]string, 0, 2)

	if s := formatSection("🕒 **МР, которые скоро попадут в проверку**", young, "Скоро попадёт в проверку", checkedAt); s != "" {
		sections = append(sections, s)
	}
	if s := formatSection("⚠️ **МР без достаточного количества ревью**", problems, "Всего найдено", checkedAt); s != "" {
		sections = append(sections, s)
	}

	return strings.Join(sections, "\n\n")
}

// formatSection рендерит один раздел сообщения: заголовок, маркированный
// список МР и итоговую строку с количеством. Возвращает пустую строку, если
// items пуст (раздел не рендерится вовсе).
func formatSection(heading string, items []MRSummary, footerLabel string, checkedAt time.Time) string {
	if len(items) == 0 {
		return ""
	}

	bullets := make([]string, 0, len(items))
	for _, m := range items {
		bullets = append(bullets, formatMRBullet(m, checkedAt))
	}

	var b strings.Builder
	b.WriteString(heading)
	b.WriteString("\n\n")
	b.WriteString(strings.Join(bullets, "\n\n"))
	fmt.Fprintf(&b, "\n\n%s: %d", footerLabel, len(items))

	return b.String()
}

// formatMRBullet рендерит один пункт маркированного списка — карточку МР.
func formatMRBullet(m MRSummary, checkedAt time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- **%s** !%d — [%s](%s)\n", m.Repository, m.IID, m.Title, m.WebURL)
	fmt.Fprintf(&b, "  Автор: @%s · создан %s · статус: %s\n", m.Author, humanizeAge(checkedAt, m.CreatedAt), m.Status)
	fmt.Fprintf(&b, "  ревьюеров: %d/%d%s · аппрувов: %d · тредов: %d (исправлено %d/%d)",
		len(m.Reviewers), m.MinReviewers, formatReviewers(m.Reviewers), m.ApprovalsCount, m.ThreadsTotal, m.ThreadsResolved, m.ThreadsTotal)
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
