package marketplace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchCurseForge(t *testing.T) {
	var requestedGameID string
	// Mock CurseForge server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-cf-key" {
			http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if strings.Contains(r.URL.Path, "/categories") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": [
					{"id": 4559, "gameId": 78022, "name": "Addons", "slug": "addons", "isClass": true},
					{"id": 12, "gameId": 78022, "name": "Resource Packs", "slug": "texture-packs", "isClass": true}
				]
			}`))
			return
		}
		if strings.Contains(r.URL.Path, "/mods/search") {
			requestedGameID = r.URL.Query().Get("gameId")
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
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	origCFURL := curseForgeBaseURL
	curseForgeBaseURL = server.URL
	defer func() { curseForgeBaseURL = origCFURL }()

	client := server.Client()

	// Test missing API key
	_, err := SearchCurseForge(context.Background(), client, "", "backpacks", "", 1, 10)
	if err == nil {
		t.Fatal("expected error with missing API key")
	}

	// Test valid search
	res, err := SearchCurseForge(context.Background(), client, "test-cf-key", "backpacks", "addon", 1, 10)
	if err != nil {
		t.Fatalf("SearchCurseForge failed: %v", err)
	}

	if requestedGameID != "78022" {
		t.Fatalf("expected gameId 78022 for Bedrock, got %s", requestedGameID)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}
	if res.Items[0].Name != "Super Backpacks" {
		t.Fatalf("expected item name Super Backpacks, got %s", res.Items[0].Name)
	}
}

func TestGetCurseForgeModDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-cf-key" {
			http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/mods/12345" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": {
					"id": 12345,
					"name": "Super Backpacks",
					"summary": "Adds backpacks for Bedrock",
					"links": {"websiteUrl": "https://curseforge.com/mc/12345", "wikiUrl": "https://wiki.example.com"},
					"downloadCount": 9876,
					"logo": {"thumbnailUrl": "https://img.example.com/icon.png"},
					"authors": [{"name": "SteveCraft"}],
					"categories": [{"id": 4559, "name": "Bedrock Addons"}],
					"dateCreated": "2024-01-01T00:00:00Z",
					"dateModified": "2024-02-01T00:00:00Z",
					"screenshots": [
						{
							"id": 101,
							"modId": 12345,
							"title": "Inventory UI",
							"description": "Showing backpacks in inventory",
							"thumbnailUrl": "https://img.example.com/thumb.png",
							"url": "https://img.example.com/full.png"
						}
					],
					"latestFiles": [
						{
							"id": 999,
							"displayName": "v1.2.0",
							"fileName": "backpacks.mcaddon",
							"fileDate": "2024-02-01T00:00:00Z",
							"fileLength": 1048576,
							"downloadUrl": "https://cdn.example.com/backpacks.mcaddon",
							"gameVersions": ["1.21.0"]
						}
					]
				}
			}`))
			return
		}
		if r.URL.Path == "/mods/12345/description" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": "<h1>Super Backpacks</h1><p>Craft backpacks using leather!</p>"
			}`))
			return
		}
		if r.URL.Path == "/mods/12345/files" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": [
					{
						"id": 999,
						"displayName": "v1.2.0",
						"fileName": "backpacks.mcaddon",
						"fileDate": "2024-02-01T00:00:00Z",
						"fileLength": 1048576,
						"downloadUrl": "https://cdn.example.com/backpacks.mcaddon",
						"gameVersions": ["1.21.0"]
					}
				]
			}`))
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	origCFURL := curseForgeBaseURL
	curseForgeBaseURL = server.URL
	defer func() { curseForgeBaseURL = origCFURL }()

	client := server.Client()

	details, err := GetCurseForgeModDetails(context.Background(), client, "test-cf-key", 12345)
	if err != nil {
		t.Fatalf("GetCurseForgeModDetails failed: %v", err)
	}

	if details.Name != "Super Backpacks" {
		t.Errorf("expected name 'Super Backpacks', got '%s'", details.Name)
	}
	if !strings.Contains(details.DescriptionHTML, "<h1>Super Backpacks</h1>") {
		t.Errorf("expected DescriptionHTML to contain heading, got '%s'", details.DescriptionHTML)
	}
	if len(details.Screenshots) != 1 || details.Screenshots[0].Title != "Inventory UI" {
		t.Errorf("expected 1 screenshot with title 'Inventory UI', got %+v", details.Screenshots)
	}
	if len(details.Files) != 1 || details.Files[0].ID != 999 {
		t.Errorf("expected 1 file with ID 999, got %+v", details.Files)
	}
	if details.WikiURL != "https://wiki.example.com" {
		t.Errorf("expected WikiURL 'https://wiki.example.com', got '%s'", details.WikiURL)
	}
}
