package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	CurseForgeBaseURL = "https://api.curseforge.com/v1"
	ModrinthBaseURL   = "https://api.modrinth.com/v2"
	MinecraftGameID   = 432
	// CurseForge Bedrock Addon class ID is 4559; Resource Packs is 12.
	CurseForgeClassBedrockAddons = 4559
	CurseForgeClassResourcePacks = 12

	MaxDownloadSize = 100 << 20 // 100 MB
)

// SafeHTTPClient returns an http.Client with a timeout and IP safety checks.
func SafeHTTPClient(timeout time.Duration, allowPrivateIPs bool) *http.Client {
	dialer := &net.Dialer{
		Timeout: timeout,
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}

			if !allowPrivateIPs {
				for _, ip := range ips {
					if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
						return nil, fmt.Errorf("access to private or local network address %s is prohibited", ip.String())
					}
				}
			}

			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

// MarketplaceItem represents a unified pack/addon listing from CurseForge or Modrinth.
type MarketplaceItem struct {
	ID          string   `json:"id"`
	Provider    string   `json:"provider"` // "curseforge" | "modrinth"
	Name        string   `json:"name"`
	Summary     string   `json:"summary"`
	Author      string   `json:"author"`
	IconURL     string   `json:"icon_url"`
	Downloads   int64    `json:"downloads"`
	Version     string   `json:"version"`
	FileID      int64    `json:"file_id,omitempty"`
	FileName    string   `json:"file_name,omitempty"`
	DownloadURL string   `json:"download_url,omitempty"`
	Categories  []string `json:"categories,omitempty"`
	PageURL     string   `json:"page_url,omitempty"`
}

// SearchResult contains matching marketplace items.
type SearchResult struct {
	Provider string            `json:"provider"`
	Items    []MarketplaceItem `json:"items"`
	Total    int               `json:"total"`
}

// --- CurseForge API Types ---

type cfSearchResponse struct {
	Data       []cfMod      `json:"data"`
	Pagination cfPagination `json:"pagination"`
}

type cfMod struct {
	ID          int64       `json:"id"`
	Name        string      `json:"name"`
	Summary     string      `json:"summary"`
	Links       cfLinks     `json:"links"`
	DownloadCnt float64     `json:"downloadCount"`
	Logo        cfLogo      `json:"logo"`
	Authors     []cfAuthor  `json:"authors"`
	Categories  []cfCat     `json:"categories"`
	LatestFiles []cfFile    `json:"latestFiles"`
}

type cfLinks struct {
	WebsiteURL string `json:"websiteUrl"`
}

type cfLogo struct {
	ThumbnailURL string `json:"thumbnailUrl"`
	URL          string `json:"url"`
}

type cfAuthor struct {
	Name string `json:"name"`
}

type cfCat struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type cfFile struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"displayName"`
	FileName    string `json:"fileName"`
	DownloadURL string `json:"downloadUrl"`
}

type cfPagination struct {
	Index      int `json:"index"`
	PageSize   int `json:"pageSize"`
	ResultCnt  int `json:"resultCount"`
	TotalCount int `json:"totalCount"`
}

// SearchCurseForge queries the CurseForge Eternal API for Minecraft Bedrock addons.
func SearchCurseForge(ctx context.Context, client *http.Client, apiKey, query, category string, page, pageSize int) (*SearchResult, error) {
	if apiKey == "" {
		return nil, errors.New("curseforge_api_key is not configured")
	}
	if client == nil {
		client = SafeHTTPClient(15*time.Second, false)
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	index := (page - 1) * pageSize

	endpoint := fmt.Sprintf("%s/mods/search", CurseForgeBaseURL)
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("gameId", strconv.Itoa(MinecraftGameID))
	q.Set("pageSize", strconv.Itoa(pageSize))
	q.Set("index", strconv.Itoa(index))
	q.Set("sortField", "2") // Popularity
	q.Set("sortOrder", "desc")

	if category == "resource" {
		q.Set("classId", strconv.Itoa(CurseForgeClassResourcePacks))
	} else if category == "behavior" || category == "addon" {
		q.Set("classId", strconv.Itoa(CurseForgeClassBedrockAddons))
	} else if category == "" {
		// Default to Bedrock Addons
		q.Set("classId", strconv.Itoa(CurseForgeClassBedrockAddons))
	}

	if query != "" {
		q.Set("searchFilter", query)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("curseforge request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("invalid or unauthorized CurseForge API key")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("curseforge api error (status %d): %s", resp.StatusCode, string(body))
	}

	var cfResp cfSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return nil, fmt.Errorf("failed to parse curseforge response: %w", err)
	}

	items := make([]MarketplaceItem, 0, len(cfResp.Data))
	for _, m := range cfResp.Data {
		author := ""
		if len(m.Authors) > 0 {
			author = m.Authors[0].Name
		}
		icon := m.Logo.ThumbnailURL
		if icon == "" {
			icon = m.Logo.URL
		}

		cats := make([]string, 0, len(m.Categories))
		for _, c := range m.Categories {
			cats = append(cats, c.Name)
		}

		var fileID int64
		var fileName, downloadURL, versionStr string
		if len(m.LatestFiles) > 0 {
			f := m.LatestFiles[0]
			fileID = f.ID
			fileName = f.FileName
			downloadURL = f.DownloadURL
			versionStr = f.DisplayName
		}

		items = append(items, MarketplaceItem{
			ID:          strconv.FormatInt(m.ID, 10),
			Provider:    "curseforge",
			Name:        m.Name,
			Summary:     m.Summary,
			Author:      author,
			IconURL:     icon,
			Downloads:   int64(m.DownloadCnt),
			Version:     versionStr,
			FileID:      fileID,
			FileName:    fileName,
			DownloadURL: downloadURL,
			Categories:  cats,
			PageURL:     m.Links.WebsiteURL,
		})
	}

	return &SearchResult{
		Provider: "curseforge",
		Items:    items,
		Total:    cfResp.Pagination.TotalCount,
	}, nil
}

// GetCurseForgeDownloadURL resolves direct download URL for a file if not populated.
func GetCurseForgeDownloadURL(ctx context.Context, client *http.Client, apiKey string, modID, fileID int64) (string, error) {
	if apiKey == "" {
		return "", errors.New("curseforge_api_key is required")
	}
	if client == nil {
		client = SafeHTTPClient(15*time.Second, false)
	}

	endpoint := fmt.Sprintf("%s/mods/%d/files/%d/download-url", CurseForgeBaseURL, modID, fileID)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("curseforge file url api status %d", resp.StatusCode)
	}

	var res struct {
		Data string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if res.Data == "" {
		return "", errors.New("curseforge author has disabled third-party automated downloads for this file")
	}

	return res.Data, nil
}

// --- Modrinth API Types ---

type mrSearchResponse struct {
	Hits      []mrHit `json:"hits"`
	TotalHits int     `json:"total_hits"`
}

type mrHit struct {
	ProjectID   string   `json:"project_id"`
	ProjectType string   `json:"project_type"`
	Slug        string   `json:"slug"`
	Author      string   `json:"author"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Categories  []string `json:"categories"`
	IconURL     string   `json:"icon_url"`
	Downloads   int64    `json:"downloads"`
}

type mrVersion struct {
	ID            string       `json:"id"`
	ProjectID     string       `json:"project_id"`
	Name          string       `json:"name"`
	VersionNumber string       `json:"version_number"`
	Files         []mrFile     `json:"files"`
}

type mrFile struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Primary  bool   `json:"primary"`
	Size     int64  `json:"size"`
}

// SearchModrinth queries the Modrinth API for Bedrock addons and resource packs.
func SearchModrinth(ctx context.Context, client *http.Client, query, category string, page, pageSize int) (*SearchResult, error) {
	if client == nil {
		client = SafeHTTPClient(15*time.Second, false)
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	endpoint := fmt.Sprintf("%s/search", ModrinthBaseURL)
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("limit", strconv.Itoa(pageSize))
	q.Set("offset", strconv.Itoa(offset))
	q.Set("index", "downloads")

	// Set query or default search
	if query != "" {
		q.Set("query", query)
	} else {
		q.Set("query", "bedrock")
	}

	if category == "resource" {
		q.Set("facets", `[["project_type:resourcepack"]]`)
	} else if category == "behavior" || category == "addon" {
		q.Set("facets", `[["project_type:mod"]]`)
	}

	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Bedrock-Server-Manager/1.0 (https://github.com/AhmadShamli/Bedrock-Server-Manager)")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("modrinth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("modrinth api error (status %d): %s", resp.StatusCode, string(body))
	}

	var mrResp mrSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&mrResp); err != nil {
		return nil, fmt.Errorf("failed to parse modrinth response: %w", err)
	}

	items := make([]MarketplaceItem, 0, len(mrResp.Hits))
	for _, hit := range mrResp.Hits {
		items = append(items, MarketplaceItem{
			ID:         hit.ProjectID,
			Provider:   "modrinth",
			Name:       hit.Title,
			Summary:    hit.Description,
			Author:     hit.Author,
			IconURL:    hit.IconURL,
			Downloads:  hit.Downloads,
			Categories: hit.Categories,
			PageURL:    fmt.Sprintf("https://modrinth.com/%s/%s", hit.ProjectType, hit.Slug),
		})
	}

	return &SearchResult{
		Provider: "modrinth",
		Items:    items,
		Total:    mrResp.TotalHits,
	}, nil
}

// GetModrinthDownloadURL fetches the primary file download URL for a project's latest version.
func GetModrinthDownloadURL(ctx context.Context, client *http.Client, projectID string) (downloadURL string, fileName string, version string, err error) {
	if client == nil {
		client = SafeHTTPClient(15*time.Second, false)
	}

	endpoint := fmt.Sprintf("%s/project/%s/version", ModrinthBaseURL, url.PathEscape(projectID))
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("User-Agent", "Bedrock-Server-Manager/1.0 (https://github.com/AhmadShamli/Bedrock-Server-Manager)")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("modrinth versions error (status %d)", resp.StatusCode)
	}

	var versions []mrVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return "", "", "", err
	}

	if len(versions) == 0 {
		return "", "", "", errors.New("no releases found for this project")
	}

	latest := versions[0]
	if len(latest.Files) == 0 {
		return "", "", "", errors.New("no downloadable files in latest release")
	}

	selectedFile := latest.Files[0]
	for _, f := range latest.Files {
		if f.Primary {
			selectedFile = f
			break
		}
	}

	return selectedFile.URL, selectedFile.Filename, latest.VersionNumber, nil
}
