package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListOpenMergeRequests_Pagination(t *testing.T) {
	pages := [][]MergeRequest{
		make([]MergeRequest, perPage),
		{{IID: 999, Title: "last"}},
	}
	for i := range pages[0] {
		pages[0][i] = MergeRequest{IID: i + 1}
	}

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
			t.Errorf("missing/incorrect PRIVATE-TOKEN header")
		}
		page := r.URL.Query().Get("page")
		var result []MergeRequest
		switch page {
		case "1":
			result = pages[0]
		case "2":
			result = pages[1]
		default:
			t.Fatalf("unexpected page: %s", page)
		}
		callCount++
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	mrs, err := client.ListOpenMergeRequests(context.Background(), "group/project")
	if err != nil {
		t.Fatalf("ListOpenMergeRequests error: %v", err)
	}

	if len(mrs) != perPage+1 {
		t.Errorf("got %d MRs, want %d", len(mrs), perPage+1)
	}
	if callCount != 2 {
		t.Errorf("expected 2 pages fetched, got %d", callCount)
	}
}

func TestGetApprovals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/api/v4/projects/group%2Fproject/merge_requests/5/approvals" {
			t.Errorf("unexpected path: %s", r.URL.EscapedPath())
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"approved_by": []map[string]interface{}{
				{"user": map[string]string{"username": "petrov"}},
				{"user": map[string]string{"username": "ivanov"}},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	usernames, err := client.GetApprovals(context.Background(), "group/project", 5)
	if err != nil {
		t.Fatalf("GetApprovals error: %v", err)
	}

	if len(usernames) != 2 {
		t.Fatalf("got %d usernames, want 2", len(usernames))
	}
}

func TestListDiscussions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/api/v4/projects/group%2Fproject/merge_requests/7/discussions" {
			t.Errorf("unexpected path: %s", r.URL.EscapedPath())
		}
		discussions := []Discussion{
			{
				ID: "d1",
				Notes: []Note{
					{Author: User{Username: "sidorov"}, System: false, Resolvable: true, Resolved: false},
				},
			},
			{
				ID: "d2",
				Notes: []Note{
					{Author: User{Username: "bot"}, System: true},
				},
			},
		}
		json.NewEncoder(w).Encode(discussions)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	discussions, err := client.ListDiscussions(context.Background(), "group/project", 7)
	if err != nil {
		t.Fatalf("ListDiscussions error: %v", err)
	}

	if len(discussions) != 2 {
		t.Fatalf("got %d discussions, want 2", len(discussions))
	}
	if !discussions[0].Notes[0].Resolvable || discussions[0].Notes[0].Resolved {
		t.Errorf("unexpected resolvable/resolved state: %+v", discussions[0].Notes[0])
	}
}

func TestListGroupMembers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/api/v4/groups/group%2Four-team/members" {
			t.Errorf("unexpected path: %s", r.URL.EscapedPath())
		}
		users := []User{
			{Username: "ivanov"},
			{Username: "petrov"},
		}
		json.NewEncoder(w).Encode(users)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	members, err := client.ListGroupMembers(context.Background(), "group/our-team")
	if err != nil {
		t.Fatalf("ListGroupMembers error: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("got %d members, want 2", len(members))
	}
}

func TestGet_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	_, err := client.ListOpenMergeRequests(context.Background(), "group/project")
	if err == nil {
		t.Fatal("expected error for 502 response, got nil")
	}
}
