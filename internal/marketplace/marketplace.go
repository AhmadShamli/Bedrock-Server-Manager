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
	"strings"
	"sync"
	"time"
)

const (
	CurseForgeBaseURL = "https://api.curseforge.com/v1"

	// MinecraftBedrockGameID is CurseForge's official game ID for Minecraft Bedrock Edition.
	// Minecraft Java Edition is 432.
	MinecraftBedrockGameID = 78022

	MaxDownloadSize = 100 << 20 // 100 MB
)

var (
	curseForgeBaseURL = CurseForgeBaseURL
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

// MarketplaceItem represents a community addon listing (CurseForge).
type MarketplaceItem struct {
	ID          string   `json:"id"`
	Provider    string   `json:"provider"` // "curseforge"
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

type MarketplaceScreenshot struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ThumbnailURL string `json:"thumbnail_url"`
	URL          string `json:"url"`
}

type MarketplaceFile struct {
	ID           int64    `json:"id"`
	DisplayName  string   `json:"display_name"`
	FileName     string   `json:"file_name"`
	FileDate     string   `json:"file_date"`
	FileLength   int64    `json:"file_length"`
	DownloadURL  string   `json:"download_url,omitempty"`
	GameVersions []string `json:"game_versions,omitempty"`
}

type MarketplaceItemDetails struct {
	MarketplaceItem
	DescriptionHTML string                  `json:"description_html"`
	Screenshots     []MarketplaceScreenshot `json:"screenshots"`
	Files           []MarketplaceFile       `json:"files"`
	DateCreated     string                  `json:"date_created,omitempty"`
	DateModified    string                  `json:"date_modified,omitempty"`
	WebsiteURL      string                  `json:"website_url,omitempty"`
	WikiURL         string                  `json:"wiki_url,omitempty"`
	IssuesURL       string                  `json:"issues_url,omitempty"`
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
	ID           int64          `json:"id"`
	Name         string         `json:"name"`
	Summary      string         `json:"summary"`
	Links        cfLinks        `json:"links"`
	DownloadCnt  float64        `json:"downloadCount"`
	Logo         cfLogo         `json:"logo"`
	Authors      []cfAuthor     `json:"authors"`
	Categories   []cfCat        `json:"categories"`
	LatestFiles  []cfFile       `json:"latestFiles"`
	Screenshots  []cfScreenshot `json:"screenshots"`
	DateModified string         `json:"dateModified"`
	DateCreated  string         `json:"dateCreated"`
}

type cfScreenshot struct {
	ID           int64  `json:"id"`
	ModID        int64  `json:"modId"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ThumbnailURL string `json:"thumbnailUrl"`
	URL          string `json:"url"`
}

type cfLinks struct {
	WebsiteURL string `json:"websiteUrl"`
	WikiURL    string `json:"wikiUrl"`
	IssuesURL  string `json:"issuesUrl"`
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
	ID           int64    `json:"id"`
	DisplayName  string   `json:"displayName"`
	FileName     string   `json:"fileName"`
	FileDate     string   `json:"fileDate"`
	FileLength   int64    `json:"fileLength"`
	DownloadURL  string   `json:"downloadUrl"`
	GameVersions []string `json:"gameVersions"`
}

type cfPagination struct {
	Index      int `json:"index"`
	PageSize   int `json:"pageSize"`
	ResultCnt  int `json:"resultCount"`
	TotalCount int `json:"totalCount"`
}

// CFCategory represents a category or class returned by CurseForge.
type CFCategory struct {
	ID      int    `json:"id"`
	GameID  int    `json:"gameId"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	ClassID int    `json:"classId,omitempty"`
	IsClass bool   `json:"isClass,omitempty"`
}

var (
	cfCategoriesMu    sync.RWMutex
	cfCategoriesCache []CFCategory
	cfCategoriesExp   time.Time
)

// FetchCurseForgeCategories retrieves and caches categories for Minecraft Bedrock (gameId 78022).
func FetchCurseForgeCategories(ctx context.Context, client *http.Client, apiKey string) ([]CFCategory, error) {
	cfCategoriesMu.RLock()
	if len(cfCategoriesCache) > 0 && time.Now().Before(cfCategoriesExp) {
		cats := cfCategoriesCache
		cfCategoriesMu.RUnlock()
		return cats, nil
	}
	cfCategoriesMu.RUnlock()

	if client == nil {
		client = SafeHTTPClient(15*time.Second, false)
	}

	endpoint := fmt.Sprintf("%s/categories?gameId=%d", curseForgeBaseURL, MinecraftBedrockGameID)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("curseforge categories status %d", resp.StatusCode)
	}

	var res struct {
		Data []CFCategory `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	cfCategoriesMu.Lock()
	cfCategoriesCache = res.Data
	cfCategoriesExp = time.Now().Add(6 * time.Hour)
	cfCategoriesMu.Unlock()

	return res.Data, nil
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

	endpoint := fmt.Sprintf("%s/mods/search", curseForgeBaseURL)
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("gameId", strconv.Itoa(MinecraftBedrockGameID))
	q.Set("pageSize", strconv.Itoa(pageSize))
	q.Set("index", strconv.Itoa(index))
	q.Set("sortField", "2") // Popularity
	q.Set("sortOrder", "desc")

	// Dynamic category matching against Bedrock categories
	if category != "" && category != "all" {
		cats, _ := FetchCurseForgeCategories(ctx, client, apiKey)
		for _, c := range cats {
			lowerSlug := strings.ToLower(c.Slug)
			lowerName := strings.ToLower(c.Name)
			if category == "resource" && (strings.Contains(lowerSlug, "resource") || strings.Contains(lowerSlug, "texture") || strings.Contains(lowerName, "texture")) {
				if c.IsClass {
					q.Set("classId", strconv.Itoa(c.ID))
				} else {
					q.Set("categoryId", strconv.Itoa(c.ID))
				}
				break
			} else if (category == "behavior" || category == "addon") && (strings.Contains(lowerSlug, "addon") || strings.Contains(lowerSlug, "behavior") || strings.Contains(lowerName, "addon")) {
				if c.IsClass {
					q.Set("classId", strconv.Itoa(c.ID))
				} else {
					q.Set("categoryId", strconv.Itoa(c.ID))
				}
				break
			}
		}
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

	endpoint := fmt.Sprintf("%s/mods/%d/files/%d/download-url", curseForgeBaseURL, modID, fileID)
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

// GetCurseForgeModDetails fetches rich details for a single mod including description HTML, screenshots, and files.
func GetCurseForgeModDetails(ctx context.Context, client *http.Client, apiKey string, modID int64) (*MarketplaceItemDetails, error) {
	if apiKey == "" {
		return nil, errors.New("curseforge_api_key is required")
	}
	if client == nil {
		client = SafeHTTPClient(15*time.Second, false)
	}

	// 1. Fetch Mod Details
	modEndpoint := fmt.Sprintf("%s/mods/%d", curseForgeBaseURL, modID)
	req, err := http.NewRequestWithContext(ctx, "GET", modEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("curseforge mod details request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("invalid or unauthorized CurseForge API key")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("curseforge mod details status %d: %s", resp.StatusCode, string(body))
	}

	var modRes struct {
		Data cfMod `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modRes); err != nil {
		return nil, fmt.Errorf("failed to decode mod details: %w", err)
	}
	m := modRes.Data

	// 2. Fetch Description HTML (optional, best-effort)
	descHTML := m.Summary
	descReq, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/mods/%d/description", curseForgeBaseURL, modID), nil)
	if err == nil {
		descReq.Header.Set("x-api-key", apiKey)
		descReq.Header.Set("Accept", "application/json")
		if descResp, err := client.Do(descReq); err == nil {
			defer descResp.Body.Close()
			if descResp.StatusCode == http.StatusOK {
				var descData struct {
					Data string `json:"data"`
				}
				if err := json.NewDecoder(descResp.Body).Decode(&descData); err == nil && strings.TrimSpace(descData.Data) != "" {
					descHTML = descData.Data
				}
			}
		}
	}

	// 3. Fetch Files (optional, check /files for full list, fallback to LatestFiles)
	files := make([]MarketplaceFile, 0)
	filesReq, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/mods/%d/files?pageSize=20", curseForgeBaseURL, modID), nil)
	if err == nil {
		filesReq.Header.Set("x-api-key", apiKey)
		filesReq.Header.Set("Accept", "application/json")
		if filesResp, err := client.Do(filesReq); err == nil {
			defer filesResp.Body.Close()
			if filesResp.StatusCode == http.StatusOK {
				var filesData struct {
					Data []cfFile `json:"data"`
				}
				if err := json.NewDecoder(filesResp.Body).Decode(&filesData); err == nil && len(filesData.Data) > 0 {
					for _, f := range filesData.Data {
						files = append(files, MarketplaceFile{
							ID:           f.ID,
							DisplayName:  f.DisplayName,
							FileName:     f.FileName,
							FileDate:     f.FileDate,
							FileLength:   f.FileLength,
							DownloadURL:  f.DownloadURL,
							GameVersions: f.GameVersions,
						})
					}
				}
			}
		}
	}

	if len(files) == 0 && len(m.LatestFiles) > 0 {
		for _, f := range m.LatestFiles {
			files = append(files, MarketplaceFile{
				ID:           f.ID,
				DisplayName:  f.DisplayName,
				FileName:     f.FileName,
				FileDate:     f.FileDate,
				FileLength:   f.FileLength,
				DownloadURL:  f.DownloadURL,
				GameVersions: f.GameVersions,
			})
		}
	}

	// 4. Screenshots
	screenshots := make([]MarketplaceScreenshot, 0, len(m.Screenshots))
	for _, s := range m.Screenshots {
		screenshots = append(screenshots, MarketplaceScreenshot{
			ID:           s.ID,
			Title:        s.Title,
			Description:  s.Description,
			ThumbnailURL: s.ThumbnailURL,
			URL:          s.URL,
		})
	}

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

	var latestFileID int64
	var latestFileName, latestDownloadURL, versionStr string
	if len(files) > 0 {
		latestFileID = files[0].ID
		latestFileName = files[0].FileName
		latestDownloadURL = files[0].DownloadURL
		versionStr = files[0].DisplayName
	}

	return &MarketplaceItemDetails{
		MarketplaceItem: MarketplaceItem{
			ID:          strconv.FormatInt(m.ID, 10),
			Provider:    "curseforge",
			Name:        m.Name,
			Summary:     m.Summary,
			Author:      author,
			IconURL:     icon,
			Downloads:   int64(m.DownloadCnt),
			Version:     versionStr,
			FileID:      latestFileID,
			FileName:    latestFileName,
			DownloadURL: latestDownloadURL,
			Categories:  cats,
			PageURL:     m.Links.WebsiteURL,
		},
		DescriptionHTML: descHTML,
		Screenshots:     screenshots,
		Files:           files,
		DateCreated:     m.DateCreated,
		DateModified:    m.DateModified,
		WebsiteURL:      m.Links.WebsiteURL,
		WikiURL:         m.Links.WikiURL,
		IssuesURL:       m.Links.IssuesURL,
	}, nil
}


