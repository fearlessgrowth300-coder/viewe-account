package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net"
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

	"github.com/chromedp/chromedp"
)

var (
	activeWorkers int64
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

type ViewerJob struct {
	TargetURL string
	ProxyURL  string
}

// isValidProxyIP filters out bogon / invalid IP addresses like 0.0.0.0
func isValidProxyIP(proxyStr string) bool {
	u, err := url.Parse(proxyStr)
	if err != nil {
		return false
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if ip == nil {
		return true // Hostname (like proxy.soax.com)
	}
	if ip.IsUnspecified() || ip.IsLoopback() {
		return false
	}
	return true
}

// quickTestProxy verifies that a proxy can actually perform HTTPS requests within 3.5s
func quickTestProxy(proxyStr string, testURL string) bool {
	pURL, err := url.Parse(proxyStr)
	if err != nil {
		return false
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(pURL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 3500 * time.Millisecond,
	}

	req, err := http.NewRequest("HEAD", testURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}

// loadAndFilterProxies loads residential proxies first, then pre-tests public proxies
func loadAndFilterProxies(txtPath, jsonPath, testURL string, preFilter bool) []string {
	var verifiedPool []string
	var rawCandidates []string

	// 1. High Priority: Authenticated Residential Proxies (SOAX)
	if data, err := os.ReadFile(jsonPath); err == nil {
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
							verifiedPool = append(verifiedPool, u.String())
							fmt.Printf("[Pool] 💎 Added Premium Residential Proxy: %s (%s)\n", lp.Name, lp.City)
							continue
						}
					}
					verifiedPool = append(verifiedPool, server)
				}
			}
		}
	}

	// 2. Read public proxies from text file
	if file, err := os.Open(txtPath); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				if !strings.HasPrefix(line, "http://") && !strings.HasPrefix(line, "https://") {
					line = "http://" + line
				}
				if isValidProxyIP(line) {
					rawCandidates = append(rawCandidates, line)
				}
			}
		}
		file.Close()
	}

	if !preFilter {
		// If pre-filtering is disabled, append raw candidates directly
		return append(verifiedPool, rawCandidates...)
	}

	// 3. Pre-test candidates concurrently to eliminate dead proxies before launching Chrome
	if len(rawCandidates) > 0 {
		fmt.Printf("[Tester] 🔍 Testing up to 100 public candidates with 3.5s timeout trap...\n")
		var wg sync.WaitGroup
		var mu sync.Mutex
		sem := make(chan struct{}, 40)

		limit := 100
		if len(rawCandidates) < limit {
			limit = len(rawCandidates)
		}

		for i := 0; i < limit; i++ {
			p := rawCandidates[i]
			wg.Add(1)
			sem <- struct{}{}

			go func(proxyStr string) {
				defer wg.Done()
				defer func() { <-sem }()

				if quickTestProxy(proxyStr, testURL) {
					mu.Lock()
					verifiedPool = append(verifiedPool, proxyStr)
					mu.Unlock()
					fmt.Printf("  [💚 GREEN] Verified Proxy: %s\n", proxyStr)
				}
			}(p)
		}
		wg.Wait()
	}

	return verifiedPool
}

// runBrowserWorker initializes a headless Chrome instance with certificate bypass
func runBrowserWorker(ctx context.Context, id int, jobs <-chan ViewerJob, wg *sync.WaitGroup, sessionMinutes int) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-jobs:
			if !ok {
				return
			}

			proxyLabel := job.ProxyURL
			if proxyLabel == "" {
				proxyLabel = "Direct"
			}
			fmt.Printf("[Worker #%02d] 🌐 Launching Headless Chrome via: %s\n", id, proxyLabel)

			// 1. Chrome Flags with SSL bypass & resource optimizations
			execOpts := append(chromedp.DefaultExecAllocatorOptions[:],
				chromedp.Flag("headless", true),
				chromedp.Flag("ignore-certificate-errors", true), // Prevents ERR_CERT_AUTHORITY_INVALID
				chromedp.Flag("allow-running-insecure-content", true),
				chromedp.Flag("mute-audio", true),
				chromedp.Flag("blink-settings", "imagesEnabled=false"),
				chromedp.Flag("disable-gpu", true),
				chromedp.Flag("disable-extensions", true),
				chromedp.Flag("disable-dev-shm-usage", true),
				chromedp.Flag("no-sandbox", true),
				chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
			)

			if job.ProxyURL != "" {
				execOpts = append(execOpts, chromedp.Flag("proxy-server", job.ProxyURL))
			}

			allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, execOpts...)
			browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
			// Shorter timeout on page load so dead proxies don't freeze workers
			navCtx, cancelNav := context.WithTimeout(browserCtx, 45*time.Second)

			fmt.Printf("[Worker #%02d] Navigating to %s...\n", id, job.TargetURL)
			navErr := chromedp.Run(navCtx,
				chromedp.Navigate(job.TargetURL),
				chromedp.WaitVisible(`body`, chromedp.ByQuery),
			)
			cancelNav()

			if navErr != nil {
				fmt.Printf("[Worker #%02d] ❌ Session failed (rotating proxy): %v\n", id, navErr)
				cancelBrowser()
				cancelAlloc()
				time.Sleep(1 * time.Second)
				continue
			}

			atomic.AddInt64(&activeWorkers, 1)
			current := atomic.LoadInt64(&activeWorkers)
			fmt.Printf("[Worker #%02d] 💚 Connection active on stream! Active Browsers: %d\n", id, current)

			// 2. Keep browser session alive
			sessionDuration := time.Duration(sessionMinutes)*time.Minute + time.Duration(rand.Intn(30))*time.Second
			select {
			case <-ctx.Done():
			case <-time.After(sessionDuration):
			}

			atomic.AddInt64(&activeWorkers, -1)
			cancelBrowser()
			cancelAlloc()
			fmt.Printf("[Worker #%02d] Session completed. Cleaned up browser resources.\n", id)
		}
	}
}

func main() {
	channelFlag := flag.String("channel", "agenciapx", "Target Twitch channel username")
	urlFlag := flag.String("url", "", "Custom target URL")
	workersFlag := flag.Int("workers", 3, "Number of concurrent headless browsers")
	proxiesPath := flag.String("proxies", "proxies.txt", "Path to proxies.txt")
	jsonPath := flag.String("json-proxies", "data/proxies.json", "Path to data/proxies.json")
	sessionFlag := flag.Int("session-duration", 15, "Session duration per browser in minutes")
	totalDurationFlag := flag.Int("duration", 0, "Total runtime in minutes (0 for infinite)")
	preFilterFlag := flag.Bool("pre-filter", true, "Test public proxies before launching Chrome")
	soaxOnlyFlag := flag.Bool("soax-only", false, "Use only premium residential proxies (SOAX)")
	flag.Parse()

	targetStream := *urlFlag
	if targetStream == "" {
		channel := strings.TrimPrefix(*channelFlag, "https://www.twitch.tv/")
		channel = strings.TrimPrefix(channel, "https://twitch.tv/")
		channel = strings.Trim(channel, "/")
		targetStream = fmt.Sprintf("https://www.twitch.tv/%s", channel)
	}

	fmt.Println("==================================================================")
	fmt.Println("⚡ GO CHROMEDP HEADLESS ENGINE (WITH PRE-FLIGHT PROXY FILTER)")
	fmt.Printf("   Target Stream:     %s\n", targetStream)
	fmt.Printf("   Workers:           %d instances\n", *workersFlag)
	fmt.Printf("   Session Duration:  %d minutes\n", *sessionFlag)
	fmt.Printf("   Pre-Filter Public: %v\n", *preFilterFlag)
	fmt.Println("==================================================================")

	txtFile := *proxiesPath
	if _, err := os.Stat(txtFile); os.IsNotExist(err) {
		txtFile = filepath.Join("..", "proxies.txt")
	}
	jsonFile := *jsonPath
	if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
		jsonFile = filepath.Join("..", "data", "proxies.json")
	}

	var proxyPool []string
	if *soaxOnlyFlag {
		proxyPool = loadAndFilterProxies("", jsonFile, targetStream, false)
	} else {
		proxyPool = loadAndFilterProxies(txtFile, jsonFile, targetStream, *preFilterFlag)
	}

	if len(proxyPool) == 0 {
		fmt.Println("⚠️  No verified proxies available. Falling back to Direct.")
		proxyPool = []string{""}
	} else {
		fmt.Printf("\n🎯 Active Rotation Pool: %d verified proxies ready for workers.\n\n", len(proxyPool))
	}

	ctx, cancel := context.WithCancel(context.Background())
	if *totalDurationFlag > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*totalDurationFlag)*time.Minute)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup
	jobChannel := make(chan ViewerJob, *workersFlag*2)

	for w := 1; w <= *workersFlag; w++ {
		wg.Add(1)
		go runBrowserWorker(ctx, w, jobChannel, &wg, *sessionFlag)
	}

	go func() {
		defer close(jobChannel)
		idx := 0
		for {
			select {
			case <-ctx.Done():
				return
			case jobChannel <- ViewerJob{
				TargetURL: targetStream,
				ProxyURL:  proxyPool[idx%len(proxyPool)],
			}:
				idx++
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()

	select {
	case <-sigChan:
		fmt.Println("\n🛑 Stop signal received. Gracefully closing all browsers...")
	case <-ctx.Done():
		fmt.Println("\n⏱️ Configured runtime completed.")
	}

	cancel()
	wg.Wait()
	fmt.Println("✅ All browser instances closed cleanly.")
}
