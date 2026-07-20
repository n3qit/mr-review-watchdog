package checker

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubCalendarClient struct {
	holidays map[int][]time.Time
	err      error
}

func (s *stubCalendarClient) GetHolidays(ctx context.Context, year int) ([]time.Time, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.holidays[year], nil
}

func moscowDate(year int, month time.Month, day, hour, min int) time.Time {
	return time.Date(year, month, day, hour, min, 0, 0, moscow)
}

func TestBusinessHoursElapsed_SameWorkday(t *testing.T) {
	// Четверг 2026-07-16.
	createdAt := moscowDate(2026, 7, 16, 9, 0)
	now := moscowDate(2026, 7, 16, 15, 0)

	cache := newHolidayCache(&stubCalendarClient{})
	elapsed, err := businessHoursElapsed(context.Background(), cache, createdAt, now)
	if err != nil {
		t.Fatalf("businessHoursElapsed error: %v", err)
	}
	if elapsed != 6*time.Hour {
		t.Errorf("elapsed = %v, want 6h", elapsed)
	}
}

func TestBusinessHoursElapsed_SkipsWeekend(t *testing.T) {
	// Пятница 2026-07-17 18:00 -> понедельник 2026-07-20 10:00.
	createdAt := moscowDate(2026, 7, 17, 18, 0)
	now := moscowDate(2026, 7, 20, 10, 0)

	cache := newHolidayCache(&stubCalendarClient{})
	elapsed, err := businessHoursElapsed(context.Background(), cache, createdAt, now)
	if err != nil {
		t.Fatalf("businessHoursElapsed error: %v", err)
	}
	// Пт 18:00-24:00 (6ч) + Сб (0) + Вс (0) + Пн 00:00-10:00 (10ч) = 16ч.
	if elapsed != 16*time.Hour {
		t.Errorf("elapsed = %v, want 16h", elapsed)
	}
}

func TestBusinessHoursElapsed_SkipsHoliday(t *testing.T) {
	// Среда 2026-01-07 — праздник (Рождество). Вторник 2026-01-06 10:00 -> четверг 2026-01-08 10:00.
	createdAt := moscowDate(2026, 1, 6, 10, 0)
	now := moscowDate(2026, 1, 8, 10, 0)

	cache := newHolidayCache(&stubCalendarClient{
		holidays: map[int][]time.Time{
			2026: {time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)},
		},
	})

	elapsed, err := businessHoursElapsed(context.Background(), cache, createdAt, now)
	if err != nil {
		t.Fatalf("businessHoursElapsed error: %v", err)
	}
	// Вт 10:00-24:00 (14ч) + Ср праздник (0) + Чт 00:00-10:00 (10ч) = 24ч.
	if elapsed != 24*time.Hour {
		t.Errorf("elapsed = %v, want 24h", elapsed)
	}
}

func TestBusinessHoursElapsed_NowBeforeCreatedAt(t *testing.T) {
	createdAt := moscowDate(2026, 7, 16, 15, 0)
	now := moscowDate(2026, 7, 16, 9, 0)

	cache := newHolidayCache(&stubCalendarClient{})
	elapsed, err := businessHoursElapsed(context.Background(), cache, createdAt, now)
	if err != nil {
		t.Fatalf("businessHoursElapsed error: %v", err)
	}
	if elapsed != 0 {
		t.Errorf("elapsed = %v, want 0", elapsed)
	}
}

func TestBusinessHoursElapsed_CalendarErrorPropagates(t *testing.T) {
	createdAt := moscowDate(2026, 7, 16, 9, 0)
	now := moscowDate(2026, 7, 16, 15, 0)

	cache := newHolidayCache(&stubCalendarClient{err: errors.New("502 bad gateway")})
	_, err := businessHoursElapsed(context.Background(), cache, createdAt, now)
	if err == nil {
		t.Fatal("expected error when calendar fetch fails, got nil")
	}
}

func TestHolidayCache_FetchesEachYearOnce(t *testing.T) {
	calls := 0
	client := &countingCalendarClient{onCall: func() { calls++ }}
	cache := newHolidayCache(client)

	for _, d := range []time.Time{
		moscowDate(2026, 1, 5, 0, 0),
		moscowDate(2026, 1, 6, 0, 0),
		moscowDate(2026, 1, 7, 0, 0),
	} {
		if _, err := cache.isWorkingDay(context.Background(), d); err != nil {
			t.Fatalf("isWorkingDay error: %v", err)
		}
	}

	if calls != 1 {
		t.Errorf("expected 1 call to GetHolidays (cached per year), got %d", calls)
	}
}

type countingCalendarClient struct {
	onCall func()
}

func (c *countingCalendarClient) GetHolidays(ctx context.Context, year int) ([]time.Time, error) {
	c.onCall()
	return nil, nil
}
