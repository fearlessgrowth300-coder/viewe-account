package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
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

type ProxyEndpoint struct {
	DisplayName    string
	ChromeProxyURL string
	StopBridge     func()
}

type ViewerJob struct {
	TargetURL string
	Proxy     ProxyEndpoint
}

// readHTTPHeaderBytes reads raw header bytes until \r\n\r\n without buffering extra stream bytes
func readHTTPHeaderBytes(r io.Reader) ([]byte, error) {
	var buf []byte
	b := make([]byte, 1)
	for {
		n, err := r.Read(b)
		if n > 0 {
			buf = append(buf, b[0])
			if len(buf) >= 4 && string(buf[len(buf)-4:]) == "\r\n\r\n" {
				return buf, nil
			}
			if len(buf) > 8192 { // safety limit
				return nil, fmt.Errorf("header too large")
			}
		}
		if err != nil {
			return nil, err
		}
	}
}

// handleProxyClient safely establishes an unbuffered HTTP CONNECT tunnel to the upstream proxy
func handleProxyClient(clientConn net.Conn, upstreamHost, authHeader string) {
	defer clientConn.Close()

	headerBytes, err := readHTTPHeaderBytes(clientConn)
	if err != nil {
		return
	}

	headerStr := string(headerBytes)
	lines := strings.Split(headerStr, "\r\n")
	if len(lines) == 0 {
		return
	}

	reqLine := lines[0]
	parts := strings.Split(reqLine, " ")
	if len(parts) < 2 {
		return
	}

	method := parts[0]
	targetHost := parts[1]

	upstreamConn, err := net.DialTimeout("tcp", upstreamHost, 15*time.Second)
	if err != nil {
		return
	}
	defer upstreamConn.Close()

	if method == "CONNECT" {
		// Clean HTTP CONNECT request with Proxy-Authorization
		connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: %s\r\nProxy-Connection: Keep-Alive\r\n\r\n",
			targetHost, targetHost, authHeader)
		if _, err := upstreamConn.Write([]byte(connectReq)); err != nil {
			return
		}

		// Read upstream response without buffering TLS payload
		respBytes, err := readHTTPHeaderBytes(upstreamConn)
		if err != nil {
			return
		}

		respFirstLine := strings.Split(string(respBytes), "\r\n")[0]
		if !strings.Contains(respFirstLine, "200") {
			return
		}

		// Notify Chrome that connection is ready
		if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
			return
		}

		// Pure unbuffered bidirectional streaming
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			io.Copy(upstreamConn, clientConn)
			upstreamConn.Close()
		}()
		go func() {
			defer wg.Done()
			io.Copy(clientConn, upstreamConn)
			clientConn.Close()
		}()
		wg.Wait()
	} else {
		// Plain HTTP request
		newHeader := strings.Replace(headerStr, "\r\n\r\n", fmt.Sprintf("\r\nProxy-Authorization: %s\r\n\r\n", authHeader), 1)
		upstreamConn.Write([]byte(newHeader))
		io.Copy(clientConn, upstreamConn)
	}
}

// startLocalProxyBridge creates a local listener that handles SOAX authentication
func startLocalProxyBridge(server, username, password string) (string, func(), error) {
	cleanServer := server
	cleanServer = strings.TrimPrefix(cleanServer, "http://")
	cleanServer = strings.TrimPrefix(cleanServer, "https://")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}

	localPort := listener.Addr().(*net.TCPAddr).Port
	localProxyURL := fmt.Sprintf("http://127.0.0.1:%d", localPort)
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))

	closed := int32(0)
	stop := func() {
		if atomic.CompareAndSwapInt32(&closed, 0, 1) {
			listener.Close()
		}
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleProxyClient(conn, cleanServer, authHeader)
		}
	}()

	return localProxyURL, stop, nil
}

func loadProxies(txtPath, jsonPath string, soaxOnly bool) []ProxyEndpoint {
	var endpoints []ProxyEndpoint

	// 1. Authenticated Residential Proxies (SOAX)
	if data, err := os.ReadFile(jsonPath); err == nil {
		var localProxies []LocalProxyConfig
		if err := json.Unmarshal(data, &localProxies); err == nil {
			for _, lp := range localProxies {
				if lp.Server != "" && lp.Username != "" && lp.Password != "" {
					localURL, stopBridge, err := startLocalProxyBridge(lp.Server, lp.Username, lp.Password)
					if err == nil {
						endpoints = append(endpoints, ProxyEndpoint{
							DisplayName:    fmt.Sprintf("%s (%s, %s)", lp.Name, lp.City, lp.Country),
							ChromeProxyURL: localURL,
							StopBridge:     stopBridge,
						})
						fmt.Printf("[Bridge] 💎 Bridged SOAX Proxy %s -> %s\n", lp.Name, localURL)
					}
				}
			}
		}
	}

	if soaxOnly {
		return endpoints
	}

	// 2. Public proxies from txt file
	if file, err := os.Open(txtPath); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				if !strings.HasPrefix(line, "http://") && !strings.HasPrefix(line, "https://") {
					line = "http://" + line
				}
				u, err := url.Parse(line)
				if err == nil && u.Hostname() != "0.0.0.0" && u.Hostname() != "127.0.0.1" {
					endpoints = append(endpoints, ProxyEndpoint{
						DisplayName:    line,
						ChromeProxyURL: line,
						StopBridge:     func() {},
					})
				}
			}
		}
		file.Close()
	}

	return endpoints
}

func runBrowserWorker(ctx context.Context, id int, targetURL string, proxy ProxyEndpoint, wg *sync.WaitGroup, headless bool) {
	defer wg.Done()

	proxyURL := proxy.ChromeProxyURL
	displayName := proxy.DisplayName
	if displayName == "" {
		displayName = "Direct"
	}
	fmt.Printf("[Worker #%02d] 🌐 Launching Chrome (Headless: %v) via: %s\n", id, headless, displayName)

	execOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", headless),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("allow-running-insecure-content", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
	)

	if !headless {
		execOpts = append(execOpts, chromedp.Flag("start-maximized", true))
	} else {
		execOpts = append(execOpts, chromedp.Flag("blink-settings", "imagesEnabled=false"))
	}

	if proxyURL != "" {
		execOpts = append(execOpts, chromedp.Flag("proxy-server", proxyURL))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, execOpts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	fmt.Printf("[Worker #%02d] Navigating to %s...\n", id, targetURL)
	// Issue navigation without aggressive timeout so Chrome keeps loading naturally
	_ = chromedp.Run(browserCtx,
		chromedp.Navigate(targetURL),
	)

	atomic.AddInt64(&activeWorkers, 1)
	defer atomic.AddInt64(&activeWorkers, -1)

	current := atomic.LoadInt64(&activeWorkers)
	fmt.Printf("[Worker #%02d] 💚 Chrome window open and permanently locked! Active Windows: %d\n", id, current)

	// 5. Keep-Alive Loop: Window stays open permanently until user interrupts (Ctrl+C)
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("[Worker #%02d] Stop signal received. Closing Chrome window.\n", id)
			return
		case <-ticker.C:
			var offlineMessage string
			// Soft check if stream ended or went offline (does NOT close the window)
			_ = chromedp.Run(browserCtx,
				chromedp.Evaluate(`document.querySelector(".offline-embed, .stream-ended-indicator") ? "offline" : "live"`, &offlineMessage),
			)

			if offlineMessage == "offline" {
				fmt.Printf("[Worker #%02d] 🔴 Broadcaster went offline (window remains open).\n", id)
			}
		}
	}
}

func main() {
	channelFlag := flag.String("channel", "agenciapx", "Target Twitch channel username")
	urlFlag := flag.String("url", "", "Custom target URL")
	workersFlag := flag.Int("workers", 2, "Number of concurrent browser instances (Default: 2)")
	proxiesPath := flag.String("proxies", "proxies.txt", "Path to proxies.txt")
	jsonPath := flag.String("json-proxies", "data/proxies.json", "Path to data/proxies.json")
	sessionFlag := flag.Int("session-duration", 15, "Session duration per browser in minutes")
	totalDurationFlag := flag.Int("duration", 0, "Total runtime in minutes (0 for infinite)")
	soaxOnlyFlag := flag.Bool("soax-only", true, "Use verified residential proxies with auth bridge (Default: true)")
	headlessFlag := flag.Bool("headless", false, "Run in background without opening visible Chrome window (Default: false = VISIBLE)")
	flag.Parse()

	_ = sessionFlag

	targetStream := *urlFlag
	if targetStream == "" {
		channel := strings.TrimPrefix(*channelFlag, "https://www.twitch.tv/")
		channel = strings.TrimPrefix(channel, "https://twitch.tv/")
		channel = strings.Trim(channel, "/")
		targetStream = fmt.Sprintf("https://www.twitch.tv/%s", channel)
	}

	modeStr := "VISIBLE on your screen"
	if *headlessFlag {
		modeStr = "HEADLESS (invisible in background)"
	}

	fmt.Println("==================================================================")
	fmt.Println("⚡ GO CHROMEDP ENGINE (VISIBLE BROWSER + SOAX AUTH TUNNEL)")
	fmt.Printf("   Target Stream:     %s\n", targetStream)
	fmt.Printf("   Active Workers:    %d windows\n", *workersFlag)
	fmt.Printf("   Display Mode:      %s\n", modeStr)
	fmt.Println("   Window Persistence: Permanently open (until Ctrl+C)")
	fmt.Println("==================================================================")

	txtFile := *proxiesPath
	if _, err := os.Stat(txtFile); os.IsNotExist(err) {
		txtFile = filepath.Join("..", "proxies.txt")
	}
	jsonFile := *jsonPath
	if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
		jsonFile = filepath.Join("..", "data", "proxies.json")
	}

	proxyPool := loadProxies(txtFile, jsonFile, *soaxOnlyFlag)
	if len(proxyPool) == 0 {
		fmt.Println("⚠️  No proxies found. Launching Direct connection.")
		proxyPool = []ProxyEndpoint{{DisplayName: "Direct", ChromeProxyURL: "", StopBridge: func() {}}}
	} else {
		fmt.Printf("\n🎯 Active Rotation Pool: %d verified proxy endpoints ready.\n\n", len(proxyPool))
	}

	defer func() {
		for _, p := range proxyPool {
			if p.StopBridge != nil {
				p.StopBridge()
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	if *totalDurationFlag > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*totalDurationFlag)*time.Minute)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup

	for w := 1; w <= *workersFlag; w++ {
		wg.Add(1)
		assignedProxy := proxyPool[(w-1)%len(proxyPool)]
		go runBrowserWorker(ctx, w, targetStream, assignedProxy, &wg, *headlessFlag)

		if w < *workersFlag {
			time.Sleep(3 * time.Second) // Stagger window launches by 3 seconds
		}
	}

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
