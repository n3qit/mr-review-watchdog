package calendar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetHolidays(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/calendar/2026/holidays" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"year": 2026,
			"holidays": [
				{"date": "2026-01-01T00:00:00.000Z", "name": "Новогодние каникулы"},
				{"date": "2026-02-23T00:00:00.000Z", "name": "День защитника Отечества"}
			],
			"shortDays": [
				{"date": "2026-04-30T00:00:00.000Z", "name": "День Труда"}
			],
			"status": 200
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	holidays, err := client.GetHolidays(context.Background(), 2026)
	if err != nil {
		t.Fatalf("GetHolidays error: %v", err)
	}

	if len(holidays) != 2 {
		t.Fatalf("got %d holidays, want 2", len(holidays))
	}

	want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !holidays[0].Equal(want) {
		t.Errorf("holidays[0] = %v, want %v", holidays[0], want)
	}
}

func TestGetHolidays_InvalidYear(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"error": "Invalid year", "status": 422}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetHolidays(context.Background(), 1990)
	if err == nil {
		t.Fatal("expected error for invalid year, got nil")
	}
}

func TestNewClient_DefaultBaseURL(t *testing.T) {
	client := NewClient("")
	if client.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %q, want default %q", client.baseURL, DefaultBaseURL)
	}
}
