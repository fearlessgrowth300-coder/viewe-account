package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	TwitchGQLURL   = "https://gql.twitch.tv/gql"
	TwitchClientID = "kimne78kx3ncx6brgo4mv6wki5h1ko"
)

type ProxyConfig struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Country  string `json:"country"`
	City     string `json:"city"`
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type ViewerSession struct {
	ID          int
	ChannelName string
	Proxy       ProxyConfig
	HTTPClient  *http.Client
	DeviceID    string
}

type GQLPlaybackAccessTokenResponse struct {
	Data struct {
		StreamPlaybackAccessToken struct {
			Value     string `json:"value"`
			Signature string `json:"signature"`
		} `json:"streamPlaybackAccessToken"`
	} `json:"data"`
}

func randomDeviceID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func createProxyClient(p ProxyConfig, timeout time.Duration) (*http.Client, error) {
	transport := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:        500,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	}

	if p.Server != "" {
		proxyRaw := p.Server
		if !strings.HasPrefix(proxyRaw, "http://") && !strings.HasPrefix(proxyRaw, "https://") {
			proxyRaw = "http://" + proxyRaw
		}
		parsedURL, err := url.Parse(proxyRaw)
		if err != nil {
			return nil, err
		}

		if p.Username != "" && p.Password != "" {
			parsedURL.User = url.UserPassword(p.Username, p.Password)
		}
		transport.Proxy = http.ProxyURL(parsedURL)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}, nil
}

func (s *ViewerSession) getPlaybackAccessToken(ctx context.Context) (string, string, error) {
	query := map[string]interface{}{
		"operationName": "PlaybackAccessToken",
		"variables": map[string]interface{}{
			"isLive":     true,
			"login":      s.ChannelName,
			"isVod":      false,
			"vodID":      "",
			"playerType": "site",
		},
		"extensions": map[string]interface{}{
			"persistedQuery": map[string]interface{}{
				"version":    1,
				"sha256Hash": "0828119ded1c13477966434e15800f40b819e9d8481abbfb50d73474b7d4d70c",
			},
		},
	}

	bodyBytes, _ := json.Marshal(query)
	req, err := http.NewRequestWithContext(ctx, "POST", TwitchGQLURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Client-Id", TwitchClientID)
	req.Header.Set("X-Device-Id", s.DeviceID)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var gqlResp GQLPlaybackAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return "", "", err
	}

	token := gqlResp.Data.StreamPlaybackAccessToken.Value
	sig := gqlResp.Data.StreamPlaybackAccessToken.Signature

	if token == "" || sig == "" {
		return "", "", fmt.Errorf("stream token unavailable (stream may be offline)")
	}

	return token, sig, nil
}

func (s *ViewerSession) getMasterPlaylist(ctx context.Context, token, sig string) (string, error) {
	p := rand.Intn(999999)
	usherURL := fmt.Sprintf(
		"https://usher.ttvnw.net/api/channel/hls/%s.m3u8?client_id=%s&token=%s&sig=%s&allow_source=true&allow_audio_only=true&p=%d",
		s.ChannelName,
		TwitchClientID,
		url.QueryEscape(token),
		sig,
		p,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", usherURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Client-Id", TwitchClientID)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("usher returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://") || strings.HasPrefix(line, "http://") {
			return line, nil
		}
	}

	return "", fmt.Errorf("no media chunklist URL found in playlist")
}

var activeViewers int64

func (s *ViewerSession) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	token, sig, err := s.getPlaybackAccessToken(ctx)
	if err != nil {
		fmt.Printf("[Viewer #%04d] ❌ Token error: %v\n", s.ID, err)
		return
	}

	playlistURL, err := s.getMasterPlaylist(ctx, token, sig)
	if err != nil {
		fmt.Printf("[Viewer #%04d] ❌ Playlist error: %v\n", s.ID, err)
		return
	}

	atomic.AddInt64(&activeViewers, 1)
	current := atomic.LoadInt64(&activeViewers)
	fmt.Printf("[Viewer #%04d] ✅ Connected via %s (%s) | Active Concurrency: %d\n", s.ID, s.Proxy.Name, s.Proxy.City, current)

	ticker := time.NewTicker(time.Duration(2500+rand.Intn(1500)) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			atomic.AddInt64(&activeViewers, -1)
			return
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, "GET", playlistURL, nil)
			if err != nil {
				continue
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
			req.Header.Set("Client-Id", TwitchClientID)

			resp, err := s.HTTPClient.Do(req)
			if err != nil {
				// Re-fetch token if playlist expired
				t, sSig, errToken := s.getPlaybackAccessToken(ctx)
				if errToken == nil {
					pURL, errP := s.getMasterPlaylist(ctx, t, sSig)
					if errP == nil {
						playlistURL = pURL
					}
				}
				continue
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}
}

func loadProxies(filePath string) ([]ProxyConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var proxies []ProxyConfig
	if err := json.Unmarshal(data, &proxies); err != nil {
		return nil, err
	}
	return proxies, nil
}

func main() {
	channelFlag := flag.String("channel", "vinco_vibeslive", "Target Twitch channel username")
	viewersFlag := flag.Int("viewers", 50, "Total concurrent viewer goroutines")
	proxiesPath := flag.String("proxies", "data/proxies.json", "Path to proxies.json")
	durationFlag := flag.Int("duration", 0, "Duration to run in minutes (0 for infinite)")
	flag.Parse()

	cleanChannel := strings.TrimPrefix(*channelFlag, "https://www.twitch.tv/")
	cleanChannel = strings.TrimPrefix(cleanChannel, "https://twitch.tv/")
	cleanChannel = strings.Trim(cleanChannel, "/")

	fmt.Println("==================================================================")
	fmt.Println("⚡ GO HIGH-CONCURRENCY TWITCH PROTOCOL SWARM")
	fmt.Printf("   Target:       https://www.twitch.tv/%s\n", cleanChannel)
	fmt.Printf("   Goroutines:   %d instances\n", *viewersFlag)
	if *durationFlag > 0 {
		fmt.Printf("   Duration:     %d minutes\n", *durationFlag)
	} else {
		fmt.Printf("   Duration:     Infinite (Press Ctrl+C to stop)\n")
	}
	fmt.Println("==================================================================")

	proxies, err := loadProxies(*proxiesPath)
	if err != nil || len(proxies) == 0 {
		// Try parent directory relative path
		proxies, _ = loadProxies(filepath.Join("..", "data", "proxies.json"))
	}

	if len(proxies) == 0 {
		fmt.Println("⚠️  Warning: No proxies found. Using direct network connections.")
		proxies = []ProxyConfig{{Name: "Direct", City: "Local", Server: ""}}
	} else {
		fmt.Printf("📦 Loaded %d Residential Proxies from pool.\n\n", len(proxies))
	}

	ctx, cancel := context.WithCancel(context.Background())
	if *durationFlag > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*durationFlag)*time.Minute)
	}

	var wg sync.WaitGroup

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for i := 1; i <= *viewersFlag; i++ {
		proxy := proxies[(i-1)%len(proxies)]
		client, err := createProxyClient(proxy, 12*time.Second)
		if err != nil {
			continue
		}

		session := &ViewerSession{
			ID:          i,
			ChannelName: cleanChannel,
			Proxy:       proxy,
			HTTPClient:  client,
			DeviceID:    randomDeviceID(),
		}

		wg.Add(1)
		go session.Run(ctx, &wg)

		// Natural staggered ramp-up (50ms - 150ms per goroutine)
		time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)
	}

	fmt.Printf("\n🟢 Fleet fully deployed! Running %d concurrent goroutines.\n", *viewersFlag)

	select {
	case <-sigChan:
		fmt.Println("\n🛑 Shutdown signal received. Gracefully terminating all goroutines...")
	case <-ctx.Done():
		fmt.Println("\n⏱️ Scheduled duration reached. Stopping swarm...")
	}

	cancel()
	wg.Wait()
	fmt.Println("✅ All Go viewer routines safely terminated.")
}
