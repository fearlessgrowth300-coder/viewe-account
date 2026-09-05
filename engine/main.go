package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

type ProxyEndpoint struct {
	DisplayName   string
	ChromeProxyURL string // Unauthenticated URL passed to Chrome --proxy-server
	StopBridge     func() // Closes local auth bridge if running
}

type ViewerJob struct {
	TargetURL string
	Proxy     ProxyEndpoint
}

// handleProxyClient tunnels Chrome's HTTP/HTTPS traffic through the authenticated upstream proxy
func handleProxyClient(clientConn net.Conn, upstreamHost, authHeader string) {
	defer clientConn.Close()

	req, err := http.ReadRequest(bufio.NewReader(clientConn))
	if err != nil {
		return
	}

	upstreamConn, err := net.DialTimeout("tcp", upstreamHost, 12*time.Second)
	if err != nil {
		return
	}
	defer upstreamConn.Close()

	if req.Method == "CONNECT" {
		// Send CONNECT tunnel with Proxy-Authorization header
		connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: %s\r\nProxy-Connection: Keep-Alive\r\n\r\n",
			req.URL.Host, req.URL.Host, authHeader)
		if _, err := upstreamConn.Write([]byte(connectReq)); err != nil {
			return
		}

		resp, err := http.ReadResponse(bufio.NewReader(upstreamConn), req)
		if err != nil || resp.StatusCode != 200 {
			return
		}

		if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
			return
		}

		// Bidirectional stream copy
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
		req.Header.Set("Proxy-Authorization", authHeader)
		if err := req.Write(upstreamConn); err != nil {
			return
		}
		io.Copy(clientConn, upstreamConn)
	}
}

// startLocalProxyBridge creates a local unauthenticated bridge for proxies that require credentials
func startLocalProxyBridge(server, username, password string) (string, func(), error) {
	cleanServer := server
	if strings.HasPrefix(cleanServer, "http://") {
		cleanServer = strings.TrimPrefix(cleanServer, "http://")
	} else if strings.HasPrefix(cleanServer, "https://") {
		cleanServer = strings.TrimPrefix(cleanServer, "https://")
	}

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

// loadProxies builds proxy endpoints, bridging authenticated residential proxies
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
							DisplayName:   fmt.Sprintf("%s (%s, %s)", lp.Name, lp.City, lp.Country),
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

	// 2. Unauthenticated public proxies from txt file
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
						DisplayName:   line,
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

// runBrowserWorker manages headless Chrome sessions without auth errors
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

			proxyURL := job.Proxy.ChromeProxyURL
			displayName := job.Proxy.DisplayName
			if displayName == "" {
				displayName = "Direct"
			}
			fmt.Printf("[Worker #%02d] 🌐 Launching Headless Chrome via: %s\n", id, displayName)

			// Chrome Flags: clean proxy URL with no username/password embedded
			execOpts := append(chromedp.DefaultExecAllocatorOptions[:],
				chromedp.Flag("headless", true),
				chromedp.Flag("ignore-certificate-errors", true),
				chromedp.Flag("allow-running-insecure-content", true),
				chromedp.Flag("mute-audio", true),
				chromedp.Flag("blink-settings", "imagesEnabled=false"),
				chromedp.Flag("disable-gpu", true),
				chromedp.Flag("disable-extensions", true),
				chromedp.Flag("disable-dev-shm-usage", true),
				chromedp.Flag("no-sandbox", true),
				chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
			)

			if proxyURL != "" {
				execOpts = append(execOpts, chromedp.Flag("proxy-server", proxyURL))
			}

			allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, execOpts...)
			browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
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

			sessionDuration := time.Duration(sessionMinutes)*time.Minute + time.Duration(rand.Intn(30))*time.Second
			select {
			case <-ctx.Done():
			case <-time.After(sessionDuration):
			}

			atomic.AddInt64(&activeWorkers, -1)
			cancelBrowser()
			cancelAlloc()
			fmt.Printf("[Worker #%02d] Session completed. Browser closed.\n", id)
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
	soaxOnlyFlag := flag.Bool("soax-only", true, "Use verified residential proxies with auth bridge (Default: true)")
	flag.Parse()

	targetStream := *urlFlag
	if targetStream == "" {
		channel := strings.TrimPrefix(*channelFlag, "https://www.twitch.tv/")
		channel = strings.TrimPrefix(channel, "https://twitch.tv/")
		channel = strings.Trim(channel, "/")
		targetStream = fmt.Sprintf("https://www.twitch.tv/%s", channel)
	}

	fmt.Println("==================================================================")
	fmt.Println("⚡ GO CHROMEDP HEADLESS ENGINE (WITH LOCAL AUTH PROXY BRIDGE)")
	fmt.Printf("   Target Stream:     %s\n", targetStream)
	fmt.Printf("   Workers:           %d instances\n", *workersFlag)
	fmt.Printf("   Session Duration:  %d minutes\n", *sessionFlag)
	fmt.Printf("   SOAX Auth Bridge:  %v\n", *soaxOnlyFlag)
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

	// Defer bridge cleanups on exit
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
				Proxy:     proxyPool[idx%len(proxyPool)],
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
