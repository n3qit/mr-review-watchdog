// Package calendar реализует минимальный клиент API производственного
// календаря РФ (calendar.kuzyak.in) для получения списка праздничных дней.
package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL — публичный сервер API производственного календаря РФ.
const DefaultBaseURL = "https://calendar.kuzyak.in"

// dateLayout — формат даты в ответах API (RFC3339 с нулевым временем в UTC).
const dateLayout = "2006-01-02T15:04:05.000Z"

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
// остаются рабочими.
func (c *Client) GetHolidays(ctx context.Context, year int) ([]time.Time, error) {
	url := fmt.Sprintf("%s/api/calendar/%d/holidays", c.baseURL, year)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("формирование запроса праздников за %d год: %w", year, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("запрос праздников за %d год: %w", year, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("calendar API вернул %d для праздников за %d год", resp.StatusCode, year)
	}

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
