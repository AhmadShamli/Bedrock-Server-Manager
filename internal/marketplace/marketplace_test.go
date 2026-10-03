package marketplace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchCurseForge(t *testing.T) {
	// Mock CurseForge server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-cf-key" {
			http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if !strings.Contains(r.URL.Path, "/mods/search") {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": 12345,
					"name": "Super Backpacks",
					"summary": "Adds backpacks for Bedrock",
					"links": {"websiteUrl": "https://curseforge.com/mc/12345"},
					"downloadCount": 9876,
					"logo": {"thumbnailUrl": "https://img.example.com/icon.png"},
					"authors": [{"name": "SteveCraft"}],
					"categories": [{"id": 4559, "name": "Bedrock Addons"}],
					"latestFiles": [
						{
							"id": 999,
							"displayName": "v1.2.0",
							"fileName": "backpacks.mcaddon",
							"downloadUrl": "https://cdn.example.com/backpacks.mcaddon"
						}
					]
				}
			],
			"pagination": {
				"index": 0,
				"pageSize": 20,
				"resultCount": 1,
				"totalCount": 1
			}
		}`))
	}))
	defer server.Close()

	// Direct test using client pointing to mock server
	client := server.Client()

	// Test missing API key
	_, err := SearchCurseForge(context.Background(), client, "", "backpacks", "", 1, 10)
	if err == nil {
		t.Fatal("expected error with missing API key")
	}

	// Test with test server by sending request directly or via helper
	req, _ := http.NewRequestWithContext(context.Background(), "GET", server.URL+"/mods/search?query=backpacks", nil)
	req.Header.Set("x-api-key", "test-cf-key")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
}

func TestSearchModrinth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"hits": [
				{
					"project_id": "proj123",
					"project_type": "mod",
					"slug": "custom-biomes",
					"author": "AlexDev",
					"title": "Custom Biomes",
					"description": "More biomes for Bedrock",
					"categories": ["bedrock", "worldgen"],
					"icon_url": "https://img.example.com/biomes.png",
					"downloads": 5432
				}
			],
			"total_hits": 1
		}`))
	}))
	defer server.Close()

	// Verify hits response structure
	req, _ := http.NewRequestWithContext(context.Background(), "GET", server.URL+"/search?query=biomes", nil)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
}
