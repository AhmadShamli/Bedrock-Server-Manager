package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// UpdateInfo conveys version status and availability.
type UpdateInfo struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseURL     string `json:"release_url"`
}

// Checker checks for newer Bedrock Dedicated Server versions.
type Checker struct {
	httpClient *http.Client
}

// NewChecker creates a Checker.
func NewChecker() *Checker {
	return &Checker{
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// CheckUpdate checks the latest released docker tag or fallback metadata.
func (c *Checker) CheckUpdate(ctx context.Context, currentVersion string) (*UpdateInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/itzg/docker-minecraft-bedrock-server/releases/latest", nil)
	if err != nil {
		return &UpdateInfo{CurrentVersion: currentVersion, LatestVersion: currentVersion, UpdateAvailable: false}, nil
	}
	req.Header.Set("User-Agent", "Bedrock-Server-Manager")

	resp, err := c.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		// Fallback: If offline or rate limited, return up-to-date
		return &UpdateInfo{CurrentVersion: currentVersion, LatestVersion: currentVersion, UpdateAvailable: false}, nil
	}
	defer resp.Body.Close()

	var ghRelease struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ghRelease); err != nil {
		return &UpdateInfo{CurrentVersion: currentVersion, LatestVersion: currentVersion, UpdateAvailable: false}, nil
	}

	latest := strings.TrimPrefix(ghRelease.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")

	isNewer := false
	if current != "" && current != "latest" && latest != "" && latest != current {
		isNewer = true
	}

	return &UpdateInfo{
		CurrentVersion:  currentVersion,
		LatestVersion:   latest,
		UpdateAvailable: isNewer,
		ReleaseURL:      ghRelease.HTMLURL,
	}, nil
}
