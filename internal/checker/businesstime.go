package checker

import (
	"context"
	"fmt"
	"time"

	_ "time/tzdata" // встраивает базу часовых поясов в бинарник (для Europe/Moscow в minimal-окружениях)
)

// moscow — часовой пояс, в котором определены даты производственного
// календаря РФ.
var moscow = mustLoadMoscow()

func mustLoadMoscow() *time.Location {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		// Отказоустойчивый вариант: в РФ с 2014 года нет перехода на летнее
		// время, поэтому фиксированный UTC+3 эквивалентен Europe/Moscow.
		return time.FixedZone("MSK", 3*60*60)
	}
	return loc
}

// CalendarClient — минимальный интерфейс клиента производственного календаря,
// используемого checker'ом. Позволяет подменять реализацию моками в тестах.
type CalendarClient interface {
	GetHolidays(ctx context.Context, year int) ([]time.Time, error)
}

// holidayCache лениво загружает и кэширует список праздничных дат по годам
// в рамках одного прогона Run.
type holidayCache struct {
	client CalendarClient
	years  map[int]map[string]struct{}
}

func newHolidayCache(client CalendarClient) *holidayCache {
	return &holidayCache{
		client: client,
		years:  make(map[int]map[string]struct{}),
	}
}

func (c *holidayCache) isHoliday(ctx context.Context, date time.Time) (bool, error) {
	year := date.Year()

	set, ok := c.years[year]
	if !ok {
		holidays, err := c.client.GetHolidays(ctx, year)
		if err != nil {
			return false, fmt.Errorf("получение производственного календаря за %d год: %w", year, err)
		}

		set = make(map[string]struct{}, len(holidays))
		for _, d := range holidays {
			set[d.Format("2006-01-02")] = struct{}{}
		}
		c.years[year] = set
	}

	_, isHoliday := set[date.Format("2006-01-02")]
	return isHoliday, nil
}

// isWorkingDay сообщает, является ли дата рабочим днём: не суббота/воскресенье
// и не входит в список праздников производственного календаря РФ.
// Перенесённые рабочие выходные дни не учитываются (приближённая модель).
func (c *holidayCache) isWorkingDay(ctx context.Context, date time.Time) (bool, error) {
	weekday := date.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false, nil
	}

	holiday, err := c.isHoliday(ctx, date)
	if err != nil {
		return false, err
	}

	return !holiday, nil
}

// businessHoursElapsed возвращает количество часов между createdAt и now,
// попавших на рабочие дни (по московскому времени) — часы, пришедшиеся на
// выходные и праздники производственного календаря РФ, не учитываются.
func businessHoursElapsed(ctx context.Context, cache *holidayCache, createdAt, now time.Time) (time.Duration, error) {
	createdAt = createdAt.In(moscow)
	now = now.In(moscow)

	if !now.After(createdAt) {
		return 0, nil
	}

	var elapsed time.Duration
	dayStart := startOfDay(createdAt)

	for !dayStart.After(now) {
		dayEnd := dayStart.AddDate(0, 0, 1)

		working, err := cache.isWorkingDay(ctx, dayStart)
		if err != nil {
			return 0, err
		}

		if working {
			segmentStart := dayStart
			if createdAt.After(segmentStart) {
				segmentStart = createdAt
			}
			segmentEnd := dayEnd
			if now.Before(segmentEnd) {
				segmentEnd = now
			}
			if segmentEnd.After(segmentStart) {
				elapsed += segmentEnd.Sub(segmentStart)
			}
		}

		dayStart = dayEnd
	}

	return elapsed, nil
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
