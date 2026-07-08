package mattermost

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreatePost(t *testing.T) {
	var receivedBody createPostRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing/incorrect Authorization header: %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		json.NewEncoder(w).Encode(createPostResponse{ID: "post-123"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	postID, err := client.CreatePost(context.Background(), "chan1", "hello")
	if err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}

	if postID != "post-123" {
		t.Errorf("postID = %q, want %q", postID, "post-123")
	}
	if receivedBody.RootID != "" {
		t.Errorf("expected empty RootID for CreatePost, got %q", receivedBody.RootID)
	}
	if receivedBody.ChannelID != "chan1" || receivedBody.Message != "hello" {
		t.Errorf("unexpected request body: %+v", receivedBody)
	}
}

func TestCreateReply(t *testing.T) {
	var receivedBody createPostRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		json.NewEncoder(w).Encode(createPostResponse{ID: "post-456"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	postID, err := client.CreateReply(context.Background(), "chan1", "root-1", "reply message")
	if err != nil {
		t.Fatalf("CreateReply error: %v", err)
	}

	if postID != "post-456" {
		t.Errorf("postID = %q, want %q", postID, "post-456")
	}
	if receivedBody.RootID != "root-1" {
		t.Errorf("RootID = %q, want %q", receivedBody.RootID, "root-1")
	}
}

func TestCreatePost_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	_, err := client.CreatePost(context.Background(), "chan1", "hello")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}
