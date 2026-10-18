package bds

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	MojangServicePrimaryAPI   = "https://net.web.minecraft-services.net/api/v1.0/download/links"
	MojangServiceSecondaryAPI = "https://net-secondary.web.minecraft-services.net/api/v1.0/download/links"
	MojangDownloadPageURL     = "https://www.minecraft.net/en-us/download/server/bedrock"
	MojangUserAgent           = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	DefaultCacheDuration      = 6 * time.Hour

	FallbackReleaseVersion = "1.26.52.3"
	FallbackReleaseURL     = "https://www.minecraft.net/bedrockdedicatedserver/bin-win/bedrock-server-1.26.52.3.zip"
	FallbackPreviewVersion = "1.26.70.20"
	FallbackPreviewURL     = "https://www.minecraft.net/bedrockdedicatedserver/bin-win-preview/bedrock-server-1.26.70.20.zip"
)

var (
	winReleaseRegex = regexp.MustCompile(`https?://[a-zA-Z0-9\.\-]+(?:/bedrockdedicatedserver)?/bin-win/bedrock-server-([0-9\.]+)\.zip`)
	winPreviewRegex = regexp.MustCompile(`https?://[a-zA-Z0-9\.\-]+(?:/bedrockdedicatedserver)?/bin-win-preview/bedrock-server-([0-9\.]+)\.zip`)
	genericVerRegex = regexp.MustCompile(`bedrock-server-([0-9\.]+)\.zip`)
)

type mojangDownloadLinksResponse struct {
	Result struct {
		Links []struct {
			DownloadType string `json:"downloadType"`
			DownloadURL  string `json:"downloadUrl"`
		} `json:"links"`
	} `json:"result"`
}

// ResolvedVersion details a resolved official BDS binary package.
type ResolvedVersion struct {
	Version     string    `json:"version"`
	DownloadURL string    `json:"download_url"`
	IsPreview   bool      `json:"is_preview"`
	ResolvedAt  time.Time `json:"resolved_at"`
}

// Resolver queries Mojang for official BDS Windows binaries and caches results.
type Resolver struct {
	client        *http.Client
	cacheDuration time.Duration

	mu             sync.RWMutex
	cachedRelease  *ResolvedVersion
	cachedPreview  *ResolvedVersion
	lastResolvedAt time.Time
}

// NewResolver initializes a Mojang BDS version resolver.
func NewResolver(client *http.Client) *Resolver {
	if client == nil {
		client = &http.Client{
			Timeout: 15 * time.Second,
		}
	}
	return &Resolver{
		client:        client,
		cacheDuration: DefaultCacheDuration,
	}
}

// ResolveVersion resolves a version selector ("latest", "preview", or pinned "1.26.x") to a download URL.
func (r *Resolver) ResolveVersion(ctx context.Context, selector string, forceRefresh bool) (*ResolvedVersion, error) {
	selector = strings.ToLower(strings.TrimSpace(selector))
	if selector == "" {
		selector = "latest"
	}

	// 1. Pinned explicit version (e.g. "1.26.52.3" or "preview:1.26.70.20")
	if selector != "latest" && selector != "preview" && selector != "beta" {
		if strings.HasPrefix(selector, "preview:") {
			v := strings.TrimPrefix(selector, "preview:")
			return &ResolvedVersion{
				Version:     v,
				DownloadURL: fmt.Sprintf("https://www.minecraft.net/bedrockdedicatedserver/bin-win-preview/bedrock-server-%s.zip", v),
				IsPreview:   true,
				ResolvedAt:  time.Now().UTC(),
			}, nil
		}
		return &ResolvedVersion{
			Version:     selector,
			DownloadURL: fmt.Sprintf("https://www.minecraft.net/bedrockdedicatedserver/bin-win/bedrock-server-%s.zip", selector),
			IsPreview:   false,
			ResolvedAt:  time.Now().UTC(),
		}, nil
	}

	// 2. Check cache if not force refreshing
	r.mu.RLock()
	cachedValid := !forceRefresh && time.Since(r.lastResolvedAt) < r.cacheDuration
	var cached *ResolvedVersion
	if selector == "preview" || selector == "beta" {
		cached = r.cachedPreview
	} else {
		cached = r.cachedRelease
	}
	r.mu.RUnlock()

	if cachedValid && cached != nil {
		return cached, nil
	}

	// 3. Query Mojang download endpoints
	releaseVer, previewVer, err := r.fetchFromMojang(ctx)
	if err != nil {
		// Log warning: An unreachable Mojang API never blocks a start (Decision 16)
		log.Printf("[WARN] [BDSResolver] Failed to reach Mojang download API (%v). Using cached or fallback version.", err)
		if cached != nil {
			return cached, nil
		}
		if selector == "preview" || selector == "beta" {
			return &ResolvedVersion{
				Version:     FallbackPreviewVersion,
				DownloadURL: FallbackPreviewURL,
				IsPreview:   true,
				ResolvedAt:  time.Now().UTC(),
			}, nil
		}
		// Fallback to static baseline release
		return &ResolvedVersion{
			Version:     FallbackReleaseVersion,
			DownloadURL: FallbackReleaseURL,
			IsPreview:   false,
			ResolvedAt:  time.Now().UTC(),
		}, nil
	}

	r.mu.Lock()
	r.cachedRelease = releaseVer
	r.cachedPreview = previewVer
	r.lastResolvedAt = time.Now().UTC()
	r.mu.Unlock()

	if selector == "preview" || selector == "beta" {
		if previewVer != nil {
			return previewVer, nil
		}
		return releaseVer, nil
	}
	return releaseVer, nil
}

func (r *Resolver) fetchFromMojang(ctx context.Context) (*ResolvedVersion, *ResolvedVersion, error) {
	var errs []string

	// 1. Try official Mojang JSON Service APIs first
	apis := []string{MojangServicePrimaryAPI, MojangServiceSecondaryAPI}
	for _, apiURL := range apis {
		relVer, prevVer, err := r.fetchFromServiceAPI(ctx, apiURL)
		if err == nil && relVer != nil {
			return relVer, prevVer, nil
		}
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", apiURL, err))
		}
	}

	// 2. Fallback to HTML scraping of the download page
	relVer, prevVer, err := r.fetchFromDownloadPage(ctx)
	if err == nil && relVer != nil {
		return relVer, prevVer, nil
	}
	if err != nil {
		errs = append(errs, fmt.Sprintf("%s: %v", MojangDownloadPageURL, err))
	}

	return nil, nil, fmt.Errorf("all resolution methods failed: %s", strings.Join(errs, "; "))
}

func (r *Resolver) fetchFromServiceAPI(ctx context.Context, apiURL string) (*ResolvedVersion, *ResolvedVersion, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", MojangUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var data mojangDownloadLinksResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, nil, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	var relVer *ResolvedVersion
	var prevVer *ResolvedVersion

	for _, l := range data.Result.Links {
		switch l.DownloadType {
		case "serverBedrockWindows":
			ver := extractVersionFromURL(l.DownloadURL)
			if ver != "" {
				relVer = &ResolvedVersion{
					Version:     ver,
					DownloadURL: l.DownloadURL,
					IsPreview:   false,
					ResolvedAt:  time.Now().UTC(),
				}
			}
		case "serverBedrockPreviewWindows":
			ver := extractVersionFromURL(l.DownloadURL)
			if ver != "" {
				prevVer = &ResolvedVersion{
					Version:     ver,
					DownloadURL: l.DownloadURL,
					IsPreview:   true,
					ResolvedAt:  time.Now().UTC(),
				}
			}
		}
	}

	if relVer == nil {
		return nil, nil, fmt.Errorf("no serverBedrockWindows entry found in service API response")
	}

	return relVer, prevVer, nil
}

func (r *Resolver) fetchFromDownloadPage(ctx context.Context) (*ResolvedVersion, *ResolvedVersion, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, MojangDownloadPageURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", MojangUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("unexpected status code %d from Mojang", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}
	html := string(body)

	var relVer *ResolvedVersion
	var prevVer *ResolvedVersion

	if match := winReleaseRegex.FindStringSubmatch(html); len(match) > 1 {
		relVer = &ResolvedVersion{
			Version:     match[1],
			DownloadURL: match[0],
			IsPreview:   false,
			ResolvedAt:  time.Now().UTC(),
		}
	}

	if match := winPreviewRegex.FindStringSubmatch(html); len(match) > 1 {
		prevVer = &ResolvedVersion{
			Version:     match[1],
			DownloadURL: match[0],
			IsPreview:   true,
			ResolvedAt:  time.Now().UTC(),
		}
	}

	if relVer == nil {
		return nil, nil, fmt.Errorf("could not find Windows BDS download link in Mojang HTML response")
	}

	return relVer, prevVer, nil
}

func extractVersionFromURL(u string) string {
	if match := winReleaseRegex.FindStringSubmatch(u); len(match) > 1 {
		return match[1]
	}
	if match := winPreviewRegex.FindStringSubmatch(u); len(match) > 1 {
		return match[1]
	}
	if match := genericVerRegex.FindStringSubmatch(u); len(match) > 1 {
		return match[1]
	}
	return ""
}
