package main

import (
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

// Global pool to store verified, active "green" proxies
var (
	activeProxyPool []string
	poolMutex       sync.Mutex
	activeWorkers   int64
	totalRequests   int64
)

type LocalProxyConfig struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Country  string `json:"country"`
	City     string `json:"city"`
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// fetchPublicProxies scrapes open proxy lists and combines with local residential pool
func fetchProxies(localPath string) []string {
	var harvestedList []string

	// 1. Load authenticated residential proxies from local file
	data, err := os.ReadFile(localPath)
	if err == nil {
		var localProxies []LocalProxyConfig
		if err := json.Unmarshal(data, &localProxies); err == nil {
			for _, lp := range localProxies {
				if lp.Server != "" {
					server := lp.Server
					if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
						server = "http://" + server
					}
					if lp.Username != "" && lp.Password != "" {
						u, err := url.Parse(server)
						if err == nil {
							u.User = url.UserPassword(lp.Username, lp.Password)
							harvestedList = append(harvestedList, u.String())
							continue
						}
					}
					harvestedList = append(harvestedList, server)
				}
			}
		}
	}

	// 2. Fetch public open-source HTTP/HTTPS proxy lists
	fmt.Println("[Scraper] 🌐 Harvesting public proxies from open endpoints...")
	publicSources := []string{
		"https://api.proxyscrape.com/v2/?request=getproxies&protocol=http&timeout=5000&country=all&ssl=all&anonymity=all",
		"https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/http.txt",
		"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/http.txt",
	}

	client := &http.Client{Timeout: 6 * time.Second}
	for _, sourceURL := range publicSources {
		resp, err := client.Get(sourceURL)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err == nil {
			lines := strings.Split(string(body), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					if !strings.HasPrefix(line, "http://") && !strings.HasPrefix(line, "https://") {
						line = "http://" + line
					}
					harvestedList = append(harvestedList, line)
				}
			}
		}
	}

	fmt.Printf("[Scraper] ✅ Harvested %d total raw proxy candidates.\n", len(harvestedList))
	return harvestedList
}

// testAndFilterProxies runs 4-second health checks to filter active [GREEN 💚] proxies
func testAndFilterProxies(rawProxies []string, targetTestURL string, maxTested int) {
	fmt.Println("[Tester] 🔍 Running deep health checks with 4-second timeout trap...")
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 60) // Limit concurrent testers

	tested := 0
	for _, pStr := range rawProxies {
		if maxTested > 0 && tested >= maxTested {
			break
		}
		tested++

		wg.Add(1)
		semaphore <- struct{}{}

		go func(proxyStr string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			proxyURL, err := url.Parse(proxyStr)
			if err != nil {
				return
			}

			client := &http.Client{
				Transport: &http.Transport{
					Proxy:           http.ProxyURL(proxyURL),
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
				Timeout: 4 * time.Second, // 4-Second Timeout Trap
			}

			req, err := http.NewRequest("GET", targetTestURL, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

			resp, err := client.Do(req)
			if err != nil {
				return // Proxy is dead (RED 🔴)
			}
			resp.Body.Close()

			if resp.StatusCode == 200 || resp.StatusCode == 301 || resp.StatusCode == 302 {
				poolMutex.Lock()
				activeProxyPool = append(activeProxyPool, proxyStr)
				poolMutex.Unlock()
				fmt.Printf("  [💚 GREEN] Active Proxy: %s\n", proxyStr)
			}
		}(pStr)
	}

	wg.Wait()
	fmt.Printf("[System] 🎯 Testing complete! Built active pool of %d verified green proxies.\n\n", len(activeProxyPool))
}

// getRandomProxy safely pulls a proxy from the active verified pool
func getRandomProxy() (string, error) {
	poolMutex.Lock()
	defer poolMutex.Unlock()

	if len(activeProxyPool) == 0 {
		return "", fmt.Errorf("no active proxies available in pool")
	}

	return activeProxyPool[rand.Intn(len(activeProxyPool))], nil
}

// runTrafficSimulation manages continuous viewer sessions with automatic rotation
func runTrafficSimulation(ctx context.Context, workerID int, targetURL string, wg *sync.WaitGroup) {
	defer wg.Done()
	atomic.AddInt64(&activeWorkers, 1)
	defer atomic.AddInt64(&activeWorkers, -1)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		proxyStr, err := getRandomProxy()
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		proxyURL, err := url.Parse(proxyStr)
		if err != nil {
			continue
		}

		client := &http.Client{
			Transport: &http.Transport{
				Proxy:               http.ProxyURL(proxyURL),
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  false,
			},
			Timeout: 8 * time.Second,
		}

		req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
		if err != nil {
			continue
		}

		// Spoof desktop browser headers
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Referer", "https://www.google.com/")
		req.Header.Set("Sec-Ch-Ua", "\"Chromium\";v=\"124\", \"Google Chrome\";v=\"124\", \"Not-A.Brand\";v=\"99\"")
		req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
		req.Header.Set("Sec-Ch-Ua-Platform", "\"Windows\"")

		resp, err := client.Do(req)
		if err != nil {
			// Proxy died or dropped: automatically rotate to a different proxy
			time.Sleep(1 * time.Second)
			continue
		}

		// Instant Close Reset: Close body immediately to keep bandwidth at zero
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		reqCount := atomic.AddInt64(&totalRequests, 1)
		currentActive := atomic.LoadInt64(&activeWorkers)
		if reqCount%5 == 0 || reqCount <= 10 {
			fmt.Printf("[Worker #%03d] 🟢 Pulse active (%s) | Hits: %d | Active: %d\n", workerID, proxyURL.Host, reqCount, currentActive)
		}

		// Humanized pulse delay between session pings (3s - 7s)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(3000+rand.Intn(4000)) * time.Millisecond):
		}
	}
}

func main() {
	channelFlag := flag.String("channel", "agenciapx", "Target Twitch channel username")
	viewersFlag := flag.Int("viewers", 50, "Total concurrent traffic worker goroutines")
	proxiesPath := flag.String("proxies", "data/proxies.json", "Path to local proxies.json")
	durationFlag := flag.Int("duration", 0, "Duration to run in minutes (0 for infinite)")
	flag.Parse()

	cleanChannel := strings.TrimPrefix(*channelFlag, "https://www.twitch.tv/")
	cleanChannel = strings.TrimPrefix(cleanChannel, "https://twitch.tv/")
	cleanChannel = strings.Trim(cleanChannel, "/")
	targetStreamURL := fmt.Sprintf("https://www.twitch.tv/%s", cleanChannel)

	fmt.Println("==================================================================")
	fmt.Println("⚡ SELF-SUSTAINING GO PROXY ENGINE & TRAFFIC MANAGER")
	fmt.Printf("   Target:       %s\n", targetStreamURL)
	fmt.Printf("   Concurrency:  %d workers\n", *viewersFlag)
	if *durationFlag > 0 {
		fmt.Printf("   Duration:     %d minutes\n", *durationFlag)
	} else {
		fmt.Printf("   Duration:     Infinite (Press Ctrl+C to stop)\n")
	}
	fmt.Println("==================================================================")

	// 1. Scrape & load proxies
	localProxyPath := *proxiesPath
	if _, err := os.Stat(localProxyPath); os.IsNotExist(err) {
		localProxyPath = filepath.Join("..", "data", "proxies.json")
	}
	rawProxies := fetchProxies(localProxyPath)

	// 2. Health check and filter into green pool
	testAndFilterProxies(rawProxies, "https://www.twitch.tv", 120)

	if len(activeProxyPool) == 0 {
		fmt.Println("⚠️  Warning: No public green proxies passed check. Adding local fallback.")
		activeProxyPool = append(activeProxyPool, "")
	}

	// 3. Start workers with context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	if *durationFlag > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*durationFlag)*time.Minute)
	}

	var wg sync.WaitGroup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	fmt.Printf("[System] 🚀 Launching %d parallel workers with auto-rotation...\n\n", *viewersFlag)
	for i := 1; i <= *viewersFlag; i++ {
		wg.Add(1)
		go runTrafficSimulation(ctx, i, targetStreamURL, &wg)
		time.Sleep(time.Duration(40+rand.Intn(60)) * time.Millisecond)
	}

	select {
	case <-sigChan:
		fmt.Println("\n🛑 Stop signal received. Gracefully shutting down worker fleet...")
	case <-ctx.Done():
		fmt.Println("\n⏱️ Duration completed. Stopping workers...")
	}

	cancel()
	wg.Wait()
	fmt.Println("✅ All worker routines successfully closed.")
}
