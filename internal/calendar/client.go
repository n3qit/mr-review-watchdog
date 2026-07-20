// Package calendar реализует минимальный клиент API производственного
// календаря РФ (calendar.kuzyak.in) для получения списка праздничных дней.
package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL — публичный сервер API производственного календаря РФ.
const DefaultBaseURL = "https://calendar.kuzyak.in"

// dateLayout — формат даты в ответах API (RFC3339 с нулевым временем в UTC).
const dateLayout = "2006-01-02T15:04:05.000Z"

// maxAttempts — максимальное количество попыток запроса к calendar API
// при транзиентных ошибках (сетевые сбои, 5xx).
const maxAttempts = 3

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type holidaysResponse struct {
	Year     int          `json:"year"`
	Holidays []holidayDay `json:"holidays"`
}

type holidayDay struct {
	Date string `json:"date"`
	Name string `json:"name"`
}

// GetHolidays возвращает даты нерабочих праздничных дней указанного года.
// Предпраздничные сокращённые дни (shortDays) в результат не входят, так как
// остаются рабочими. Транзиентные ошибки (сетевые сбои, 5xx) повторяются
// до maxAttempts раз; ошибки формата запроса (4xx, например неверный год)
// не повторяются.
func (c *Client) GetHolidays(ctx context.Context, year int) ([]time.Time, error) {
	url := fmt.Sprintf("%s/api/calendar/%d/holidays", c.baseURL, year)

	resp, err := c.getWithRetry(ctx, url, year)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result holidaysResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("разбор ответа праздников за %d год: %w", year, err)
	}

	dates := make([]time.Time, 0, len(result.Holidays))
	for _, h := range result.Holidays {
		d, err := time.Parse(dateLayout, h.Date)
		if err != nil {
			return nil, fmt.Errorf("разбор даты праздника %q за %d год: %w", h.Date, year, err)
		}
		dates = append(dates, d)
	}

	return dates, nil
}

// getWithRetry выполняет GET-запрос с повторными попытками при транзиентных
// ошибках (сетевые сбои, 5xx). Ошибки 4xx возвращаются немедленно без ретрая.
func (c *Client) getWithRetry(ctx context.Context, url string, year int) (*http.Response, error) {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("формирование запроса праздников за %d год: %w", year, err)
		}

		resp, err := c.httpClient.Do(req)
		switch {
		case err != nil:
			lastErr = fmt.Errorf("запрос праздников за %d год: %w", year, err)
		case resp.StatusCode >= 500:
			lastErr = fmt.Errorf("calendar API вернул %d для праздников за %d год", resp.StatusCode, year)
			resp.Body.Close()
		case resp.StatusCode < 200 || resp.StatusCode >= 300:
			resp.Body.Close()
			return nil, fmt.Errorf("calendar API вернул %d для праздников за %d год", resp.StatusCode, year)
		default:
			return resp, nil
		}

		slog.Warn("calendar API: попытка запроса праздников не удалась",
			"year", year, "attempt", attempt, "max_attempts", maxAttempts, "error", lastErr)

		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 200 * time.Millisecond):
			}
		}
	}

	return nil, fmt.Errorf("исчерпаны попытки (%d) запроса праздников за %d год: %w", maxAttempts, year, lastErr)
}
