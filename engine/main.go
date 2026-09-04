package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
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

// Global state tracking active browser workers
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

// ViewerJob holds the execution details for a single worker thread
type ViewerJob struct {
	TargetURL string
	ProxyURL  string
}

// loadAllProxies combines proxies from proxies.txt and data/proxies.json
func loadAllProxies(txtPath, jsonPath string) []string {
	var pool []string

	// 1. Read from shared proxies.txt
	if file, err := os.Open(txtPath); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				if !strings.HasPrefix(line, "http://") && !strings.HasPrefix(line, "https://") {
					line = "http://" + line
				}
				pool = append(pool, line)
			}
		}
		file.Close()
	}

	// 2. Load authenticated residential proxies from JSON if present
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
							pool = append(pool, u.String())
							continue
						}
					}
					pool = append(pool, server)
				}
			}
		}
	}

	return pool
}

// runBrowserWorker initializes a headless Chrome instance via chromedp
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
			fmt.Printf("[Worker #%02d] 🌐 Initializing Headless Browser via: %s\n", id, proxyLabel)

			// 1. Configure lightweight Chrome arguments
			execOpts := append(chromedp.DefaultExecAllocatorOptions[:],
				chromedp.Flag("headless", true),                       // Run invisible
				chromedp.Flag("mute-audio", true),                     // Mute sound to save CPU
				chromedp.Flag("blink-settings", "imagesEnabled=false"), // Turn off image rendering to save RAM
				chromedp.Flag("disable-gpu", true),                    // Prevent GPU overhead
				chromedp.Flag("disable-extensions", true),
				chromedp.Flag("disable-dev-shm-usage", true),
				chromedp.Flag("no-sandbox", true),
				chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
			)

			if job.ProxyURL != "" {
				execOpts = append(execOpts, chromedp.Flag("proxy-server", job.ProxyURL))
			}

			// 2. Create Allocator and Browser Contexts
			allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, execOpts...)
			browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
			sessionCtx, cancelSession := context.WithTimeout(browserCtx, time.Duration(sessionMinutes+5)*time.Minute)

			// 3. Navigate to target and wait for page body
			fmt.Printf("[Worker #%02d] Navigating to %s...\n", id, job.TargetURL)
			navErr := chromedp.Run(sessionCtx,
				chromedp.Navigate(job.TargetURL),
				chromedp.WaitVisible(`body`, chromedp.ByQuery),
			)

			if navErr != nil {
				fmt.Printf("[Worker #%02d] ❌ Browser session failed: %v\n", id, navErr)
				cancelSession()
				cancelBrowser()
				cancelAlloc()
				time.Sleep(2 * time.Second)
				continue
			}

			atomic.AddInt64(&activeWorkers, 1)
			current := atomic.LoadInt64(&activeWorkers)
			fmt.Printf("[Worker #%02d] 💚 Session established on stream! Active Headless Browsers: %d\n", id, current)

			// 4. Keep-Alive loop for session duration
			duration := time.Duration(sessionMinutes)*time.Minute + time.Duration(rand.Intn(60))*time.Second
			select {
			case <-ctx.Done():
			case <-time.After(duration):
			}

			atomic.AddInt64(&activeWorkers, -1)
			cancelSession()
			cancelBrowser()
			cancelAlloc()
			fmt.Printf("[Worker #%02d] Session finished. Cleaned up browser resources.\n", id)
		}
	}
}

func main() {
	channelFlag := flag.String("channel", "agenciapx", "Target Twitch channel username")
	urlFlag := flag.String("url", "", "Custom target URL (overrides -channel)")
	workersFlag := flag.Int("workers", 3, "Number of concurrent headless browsers (recommended: 2-5 per machine)")
	proxiesPath := flag.String("proxies", "proxies.txt", "Path to proxies.txt")
	jsonPath := flag.String("json-proxies", "data/proxies.json", "Path to data/proxies.json")
	sessionFlag := flag.Int("session-duration", 15, "Session duration per browser instance in minutes")
	totalDurationFlag := flag.Int("duration", 0, "Total swarm runtime in minutes (0 for infinite)")
	flag.Parse()

	targetStream := *urlFlag
	if targetStream == "" {
		channel := strings.TrimPrefix(*channelFlag, "https://www.twitch.tv/")
		channel = strings.TrimPrefix(channel, "https://twitch.tv/")
		channel = strings.Trim(channel, "/")
		targetStream = fmt.Sprintf("https://www.twitch.tv/%s", channel)
	}

	fmt.Println("==================================================================")
	fmt.Println("⚡ GO CHROMEDP HEADLESS BROWSER SWARM")
	fmt.Printf("   Target Stream:     %s\n", targetStream)
	fmt.Printf("   Headless Workers:  %d instances\n", *workersFlag)
	fmt.Printf("   Session Duration:  %d minutes per cycle\n", *sessionFlag)
	if *totalDurationFlag > 0 {
		fmt.Printf("   Total Runtime:     %d minutes\n", *totalDurationFlag)
	} else {
		fmt.Printf("   Total Runtime:     Continuous (Press Ctrl+C to stop)\n")
	}
	fmt.Println("==================================================================")

	// Resolve proxy files
	txtFile := *proxiesPath
	if _, err := os.Stat(txtFile); os.IsNotExist(err) {
		txtFile = filepath.Join("..", "proxies.txt")
	}
	jsonFile := *jsonPath
	if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
		jsonFile = filepath.Join("..", "data", "proxies.json")
	}

	proxyPool := loadAllProxies(txtFile, jsonFile)
	if len(proxyPool) == 0 {
		fmt.Println("⚠️  No proxies found in files. Running with Direct connection.")
		proxyPool = []string{""}
	} else {
		fmt.Printf("📦 Loaded %d proxies into rotation pool.\n\n", len(proxyPool))
	}

	// Setup root context
	ctx, cancel := context.WithCancel(context.Background())
	if *totalDurationFlag > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*totalDurationFlag)*time.Minute)
	}

	// Catch interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup
	jobChannel := make(chan ViewerJob, *workersFlag*2)

	// Launch worker fleet
	for w := 1; w <= *workersFlag; w++ {
		wg.Add(1)
		go runBrowserWorker(ctx, w, jobChannel, &wg, *sessionFlag)
	}

	// Feed job queue continuously
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
				time.Sleep(1 * time.Second)
			}
		}
	}()

	select {
	case <-sigChan:
		fmt.Println("\n🛑 Stop signal received. Gracefully closing all headless browsers...")
	case <-ctx.Done():
		fmt.Println("\n⏱️ Configured runtime completed. Shutting down...")
	}

	cancel()
	wg.Wait()
	fmt.Println("✅ All headless browser instances terminated cleanly.")
}
