package bds

import (
	"context"
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
	MojangDownloadPageURL = "https://www.minecraft.net/en-us/download/server/bedrock"
	MojangUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	DefaultCacheDuration  = 6 * time.Hour

	FallbackReleaseVersion = "1.21.50.07"
	FallbackReleaseURL     = "https://minecraft.azureedge.net/bin-win/bedrock-server-1.21.50.07.zip"
)

var (
	winReleaseRegex = regexp.MustCompile(`https://minecraft\.azureedge\.net/bin-win/bedrock-server-([0-9\.]+)\.zip`)
	winPreviewRegex = regexp.MustCompile(`https://minecraft\.azureedge\.net/bin-win-preview/bedrock-server-([0-9\.]+)\.zip`)
)

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

// ResolveVersion resolves a version selector ("latest", "preview", or pinned "1.21.x") to a download URL.
func (r *Resolver) ResolveVersion(ctx context.Context, selector string, forceRefresh bool) (*ResolvedVersion, error) {
	selector = strings.ToLower(strings.TrimSpace(selector))
	if selector == "" {
		selector = "latest"
	}

	// 1. Pinned explicit version (e.g. "1.21.50.07" or "preview:1.21.60.20")
	if selector != "latest" && selector != "preview" && selector != "beta" {
		if strings.HasPrefix(selector, "preview:") {
			v := strings.TrimPrefix(selector, "preview:")
			return &ResolvedVersion{
				Version:     v,
				DownloadURL: fmt.Sprintf("https://minecraft.azureedge.net/bin-win-preview/bedrock-server-%s.zip", v),
				IsPreview:   true,
				ResolvedAt:  time.Now().UTC(),
			}, nil
		}
		return &ResolvedVersion{
			Version:     selector,
			DownloadURL: fmt.Sprintf("https://minecraft.azureedge.net/bin-win/bedrock-server-%s.zip", selector),
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

	// 3. Query Mojang download page
	releaseVer, previewVer, err := r.fetchFromMojang(ctx)
	if err != nil {
		// Log warning: An unreachable Mojang API never blocks a start (Decision 16)
		log.Printf("[WARN] [BDSResolver] Failed to reach Mojang download API (%v). Using cached or fallback version.", err)
		if cached != nil {
			return cached, nil
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
