package main

import (
	"bufio"
	"bytes"
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

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

var (
	activeWorkers int64

	chatMessageBank = []string{
		"W stream let's gooo 🔥",
		"insane play right there haha",
		"stream looking super clean today",
		"LET'S GOOOO!",
		"vibes are so good in here 🙌",
		"ggwp",
		"that was actually crazy lol",
		"facts 100%",
		"no way you survived that",
		"drop the build/settings!",
		"yoooo haha",
	}
)

type CookieData struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Secure   bool    `json:"secure"`
	HTTPOnly bool    `json:"httpOnly"`
	SameSite string  `json:"sameSite"`
	Expires  float64 `json:"expires"`
}

type UserAccount struct {
	Username      string       `json:"username"`
	Password      string       `json:"password"`
	AuthToken     string       `json:"auth_token"`
	Cookies       []CookieData `json:"cookies"`
	AssignedProxy string       `json:"assigned_proxy"` // Keeps the same IP for the same account
}

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

func loadUserAccounts(accountsDir string, filterUsername string) []UserAccount {
	var accounts []UserAccount

	dir := accountsDir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		dir = filepath.Join("..", accountsDir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return accounts
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var acc UserAccount
		if err := json.Unmarshal(data, &acc); err != nil {
			continue
		}
		if acc.Username == "" {
			acc.Username = strings.TrimSuffix(entry.Name(), ".json")
		}
		if len(acc.Cookies) == 0 && acc.AuthToken == "" {
			continue
		}
		if filterUsername != "" && !strings.EqualFold(acc.Username, filterUsername) {
			continue
		}
		accounts = append(accounts, acc)
	}

	return accounts
}

// Strict IP Lock-in: ensures the same account always uses the same proxy endpoint
func getAccountProxy(account *UserAccount, proxyPool []ProxyEndpoint) ProxyEndpoint {
	if len(proxyPool) == 0 {
		return ProxyEndpoint{DisplayName: "Direct", ChromeProxyURL: "", StopBridge: func() {}}
	}
	if account != nil && account.AssignedProxy != "" {
		for _, p := range proxyPool {
			if strings.Contains(p.DisplayName, account.AssignedProxy) || strings.Contains(p.ChromeProxyURL, account.AssignedProxy) {
				return p
			}
		}
	}
	// Stable deterministic hash based on username to guarantee strict IP persistence across runs
	h := 0
	if account != nil && account.Username != "" {
		for _, c := range account.Username {
			h = (h*31 + int(c)) & 0x7fffffff
		}
	}
	assigned := proxyPool[h%len(proxyPool)]
	if account != nil {
		account.AssignedProxy = assigned.DisplayName
	}
	return assigned
}

// ExtractChannelID locates the target broadcaster's numeric ID using Twitch's page state, in-tab fetch, or direct GQL query
func ExtractChannelID(ctx context.Context, id int, channelLogin string) (string, error) {
	fmt.Printf("[Worker #%02d] 🔍 Resolving target broadcaster ID for '%s'...\n", id, channelLogin)
	var channelID string

	// 1. In-browser GQL query: query user(login: "...") { id } using the page's fetch context
	inTabGQL := fmt.Sprintf(`(async () => {
		try {
			const clientId = (window.__twilightSettings && window.__twilightSettings.clientId) ? window.__twilightSettings.clientId : "kimne78kx3ncx6brgo4mv6wki5h1ko";
			const res = await fetch("https://gql.twitch.tv/gql", {
				method: "POST",
				headers: {
					"Client-Id": clientId,
					"Content-Type": "application/json"
				},
				body: JSON.stringify([{
					"query": "query($login: String!) { user(login: $login) { id login displayName } }",
					"variables": { "login": "%s" }
				}])
			});
			const data = await res.json();
			if (data && data[0] && data[0].data && data[0].data.user && data[0].data.user.id) {
				return data[0].data.user.id;
			}
		} catch(e) {}
		return "";
	})()`, channelLogin)

	_ = chromedp.Run(ctx, chromedp.Evaluate(inTabGQL, &channelID))
	if channelID != "" {
		fmt.Printf("[Worker #%02d] 🎯 Discovered target Channel ID via in-tab GQL: %s\n", id, channelID)
		return channelID, nil
	}

	// 2. Try extracting from Twitch internal Apollo/Twilight script tags or metadata
	extractScript := `(() => {
		try {
			// Check window state or apollo cache
			if (window.__INITIAL_STATE__ && window.__INITIAL_STATE__.channelID) {
				return window.__INITIAL_STATE__.channelID.toString();
			}
			// Search script tags containing channelID
			const scripts = Array.from(document.querySelectorAll('script'));
			for (const s of scripts) {
				const m = s.textContent.match(/"channelID"\s*:\s*"(\d+)"/) || s.textContent.match(/"targetID"\s*:\s*"(\d+)"/) || s.textContent.match(/"id"\s*:\s*"(\d+)","login"/);
				if (m && m[1]) return m[1];
			}
			// Search meta tag twitter:app:url:googleplay (format: twitch://stream/{login}?channel_id={id})
			const meta = document.querySelector('meta[property="twitter:app:url:googleplay"], meta[name="twitter:app:url:googleplay"]');
			if (meta && meta.content) {
				const match = meta.content.match(/channel_id=(\d+)/) || meta.content.match(/\/channel\/(\d+)/);
				if (match && match[1]) return match[1];
			}
			// Search player or container attributes
			const player = document.querySelector('[data-channel-id]');
			if (player && player.getAttribute('data-channel-id')) {
				return player.getAttribute('data-channel-id');
			}
		} catch (e) {}
		return "";
	})()`

	_ = chromedp.Run(ctx, chromedp.Evaluate(extractScript, &channelID))
	if channelID != "" {
		fmt.Printf("[Worker #%02d] 🎯 Discovered target Channel ID via page DOM/Metadata: %s\n", id, channelID)
		return channelID, nil
	}

	// 3. Fallback direct HTTP GQL request from Go engine
	if channelLogin != "" {
		gqlPayload := []byte(fmt.Sprintf(`[{"query":"query($login: String!) { user(login: $login) { id login } }","variables":{"login":"%s"}}]`, channelLogin))
		req, err := http.NewRequestWithContext(ctx, "POST", "https://gql.twitch.tv/gql", bytes.NewBuffer(gqlPayload))
		if err == nil {
			req.Header.Set("Client-ID", "kimne78kx3ncx6brgo4mv6wki5h1ko")
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: 8 * time.Second}
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				var gqlResp []struct {
					Data struct {
						User struct {
							ID string `json:"id"`
						} `json:"user"`
					} `json:"data"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err == nil && len(gqlResp) > 0 && gqlResp[0].Data.User.ID != "" {
					channelID = gqlResp[0].Data.User.ID
					fmt.Printf("[Worker #%02d] 🎯 Discovered target Channel ID via direct Twitch GQL endpoint: %s\n", id, channelID)
					return channelID, nil
				}
			}
		}
	}

	return "", fmt.Errorf("unable to resolve numeric channel ID for '%s'", channelLogin)
}

// SendNativeFollowMutation bypasses the physical button entirely and runs the request inside Chrome's authenticated network session
func SendNativeFollowMutation(ctx context.Context, id int, targetChannelID string) error {
	fmt.Printf("[Worker #%02d] ⚡ Bypassing button interface. Injecting native GQL mutation...\n", id)

	mutationScript := fmt.Sprintf(`(async () => {
		try {
			const clientId = (window.__twilightSettings && window.__twilightSettings.clientId) ? window.__twilightSettings.clientId : "kimne78kx3ncx6brgo4mv6wki5h1ko";
			const authMatch = document.cookie.match(/auth-token=([^;]+)/);
			let authToken = authMatch ? authMatch[1] : "";
			if (!authToken) {
				authToken = localStorage.getItem('auth-token') || "";
			}

			const payload = [{
				"operationName": "FollowButton_FollowUser",
				"variables": {
					"input": {
						"disableNotifications": false,
						"targetID": "%s"
					}
				},
				"extensions": {
					"persistedQuery": {
						"version": 1,
						"sha256Hash": "38009b1191060938ff5d82998a442d9df6554b7c8df8e98b049d56075c3db731"
					}
				}
			}];

			// Extract Client-Integrity token and Device ID from page memory if present
			let integrityToken = "";
			try {
				if (window.__twilightSettings && window.__twilightSettings.integrityToken) {
					integrityToken = window.__twilightSettings.integrityToken;
				}
			} catch(e) {}

			let deviceId = "";
			const devMatch = document.cookie.match(/unique_id=([^;]+)/);
			if (devMatch) deviceId = devMatch[1];

			const headers = {
				"Client-Id": clientId,
				"Authorization": "OAuth " + authToken,
				"Content-Type": "application/json"
			};
			if (integrityToken) headers["Client-Integrity"] = integrityToken;
			if (deviceId) headers["X-Device-Id"] = deviceId;

			const res = await fetch("https://gql.twitch.tv/gql", {
				method: "POST",
				headers: headers,
				body: JSON.stringify(payload)
			});

			const text = await res.text();
			return {
				status: res.status,
				body: text
			};
		} catch (e) {
			return {
				status: 0,
				body: (e.name ? e.name + ": " : "") + (e.message || e.toString()) + (e.stack ? "\n" + e.stack : "")
			};
		}
	})()`, targetChannelID)

	var result struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}

	if err := chromedp.Run(ctx, chromedp.Evaluate(mutationScript, &result)); err != nil {
		return fmt.Errorf("native follow mutation evaluation error: %v", err)
	}

	fmt.Printf("\n[Worker #%02d] ================= GQL FOLLOW DIAGNOSTICS ================\n", id)
	fmt.Printf("   • Transport HTTP Status: %d\n", result.Status)
	fmt.Printf("   • Server Response / Error: %s\n", result.Body)
	fmt.Printf("   ==================================================================\n\n")

	// Check response content
	if strings.Contains(result.Body, `"followUser"`) || strings.Contains(result.Body, `"targetID"`) || strings.Contains(result.Body, `"following":true`) {
		fmt.Printf("[Worker #%02d] 💚 [Success] Native GQL Follow mutation accepted by Twitch servers!\n", id)
		return nil
	}

	if strings.Contains(result.Body, `"errors"`) {
		return fmt.Errorf("Twitch GQL error: %s", result.Body)
	}

	if result.Status == 200 {
		fmt.Printf("[Worker #%02d] 💚 [Success] Follow payload processed (HTTP 200).\n", id)
		return nil
	}

	return fmt.Errorf("fetch failed with status %d: %s", result.Status, result.Body)
}

// performAccountActions manages the lifecycle of an authenticated worker window
func performAccountActions(ctx context.Context, id int, account *UserAccount, targetURL string, shouldFollow bool) error {
	if account == nil {
		fmt.Printf("[Worker #%02d] Navigating to %s (Anonymous Viewer)...\n", id, targetURL)
		return chromedp.Run(ctx, chromedp.Navigate(targetURL))
	}

	// 1. Inject pre-saved authentication cookies before navigation with explicit URL
	fmt.Printf("[Worker #%02d] 🔑 Injecting %d session cookies for @%s...\n", id, len(account.Cookies), account.Username)
	err := chromedp.Run(ctx,
		network.Enable(),
		chromedp.ActionFunc(func(actCtx context.Context) error {
			for _, c := range account.Cookies {
				expr := network.SetCookie(c.Name, c.Value).
					WithURL("https://www.twitch.tv").
					WithDomain(c.Domain).
					WithPath(c.Path).
					WithSecure(c.Secure).
					WithHTTPOnly(c.HTTPOnly)
				if c.Expires > 0 {
					t := cdp.TimeSinceEpoch(time.Unix(int64(c.Expires), 0))
					expr = expr.WithExpires(&t)
				}
				_ = expr.Do(actCtx)
			}
			return nil
		}),
	)
	if err != nil {
		fmt.Printf("[Worker #%02d] ⚠️ Cookie injection notice: %v\n", id, err)
	}

	// 2. Navigate to the channel with multi-attempt retry on ERR_EMPTY_RESPONSE
	fmt.Printf("[Worker #%02d] Navigating to stream channel with account session: %s\n", id, targetURL)

	for attempt := 1; attempt <= 3; attempt++ {
		navErr := chromedp.Run(ctx,
			chromedp.Navigate(targetURL),
			chromedp.Sleep(6*time.Second),
		)

		var pageError bool
		_ = chromedp.Run(ctx, chromedp.Evaluate(`(() => {
			const title = document.title.toLowerCase();
			const body = document.body ? document.body.innerText.toLowerCase() : "";
			return title.includes("isn't working") || body.includes("err_empty_response") || body.includes("didn't send any data") || title.includes("error");
		})()`, &pageError))

		if navErr != nil || pageError {
			fmt.Printf("[Worker #%02d] 🔄 Detected ERR_EMPTY_RESPONSE / handshake delay (Attempt %d/3). Refreshing in 3s...\n", id, attempt)
			time.Sleep(3 * time.Second)
			if attempt < 3 {
				continue
			}
		} else {
			fmt.Printf("[Worker #%02d] 🌐 Stream channel loaded cleanly.\n", id)
			break
		}
	}

	// Inject LocalStorage tokens so Twitch client recognizes active login state
	if account.AuthToken != "" {
		jsInject := fmt.Sprintf(`
			try {
				localStorage.setItem('auth-token', '%s');
				localStorage.setItem('twilight.oauth.token', '%s');
				localStorage.setItem('login', '%s');
			} catch(e) {}
		`, account.AuthToken, account.AuthToken, account.Username)
		_ = chromedp.Run(ctx, chromedp.Evaluate(jsInject, nil))
	}

	// Check if user menu or login is confirmed
	var checkLogin string
	_ = chromedp.Run(ctx,
		chromedp.Evaluate(`(() => {
			const userMenu = document.querySelector('button[data-a-target="user-menu-toggle"]');
			if (userMenu) return userMenu.getAttribute('aria-label') || "Logged In User Menu";
			const loginItem = localStorage.getItem('login');
			return loginItem ? ("@" + loginItem) : "";
		})()`, &checkLogin),
	)

	if checkLogin != "" {
		fmt.Printf("[Worker #%02d] 👤 Confirmed Authenticated Session: %s\n", id, checkLogin)
	}

	// Dismiss consent banners or mature stream dialogs if present
	_ = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const matureBtn = document.querySelector('button[data-a-target="player-overlay-mature-accept"], button:has-text("Start Watching")');
				if (matureBtn) matureBtn.click();
				const consentBtn = document.querySelector('button[data-a-target="consent-banner-accept"]');
				if (consentBtn) consentBtn.click();
			})()
		`, nil),
	)

	// 3. Execute Native Follow Mutation Bypassing Button Interface
	if shouldFollow {
		// Human Warmup: allow stream video player and integrity handshake to settle
		fmt.Printf("[Worker #%02d] 🕒 Simulating human viewer warmup (5s) before follow action...\n", id)
		time.Sleep(5 * time.Second)

		// 1. Extract Target Channel Login Name from Target URL
		channelLogin := targetURL
		channelLogin = strings.TrimPrefix(channelLogin, "https://www.twitch.tv/")
		channelLogin = strings.TrimPrefix(channelLogin, "https://twitch.tv/")
		channelLogin = strings.Trim(channelLogin, "/")

		targetChannelID, err := ExtractChannelID(ctx, id, channelLogin)
		if err != nil {
			fmt.Printf("[Worker #%02d] ⚠️ Channel ID extraction notice: %v\n", id, err)
		} else {
			// 2. Inject Native Authorized GraphQL Follow Mutation
			if err := SendNativeFollowMutation(ctx, id, targetChannelID); err != nil {
				fmt.Printf("[Worker #%02d] ⚠️ Native mutation notice (%v). Engaging physical UI button click as fallback...\n", id, err)

				followButtonSelector := `button[data-a-target="follow-button"]`
				_ = chromedp.Run(ctx,
					chromedp.ScrollIntoView(followButtonSelector, chromedp.ByQuery),
					chromedp.Sleep(500*time.Millisecond),
					chromedp.ActionFunc(func(actCtx context.Context) error {
						return chromedp.Evaluate(fmt.Sprintf(`(() => {
							const btn = document.querySelector('%s');
							if (!btn) return false;
							['pointerdown', 'mousedown', 'pointerup', 'mouseup'].forEach(evt => {
								btn.dispatchEvent(new MouseEvent(evt, { view: window, bubbles: true, cancelable: true, buttons: 1 }));
							});
							btn.click();
							return true;
						})()`, followButtonSelector), nil).Do(actCtx)
					}),
					chromedp.Sleep(3*time.Second),
				)
			} else {
				// 3. Update DOM/UI state to reflect following if present
				_ = chromedp.Run(ctx,
					chromedp.Sleep(1*time.Second),
					chromedp.Evaluate(`(() => {
						const followBtn = document.querySelector('button[data-a-target="follow-button"]');
						if (followBtn) {
							followBtn.setAttribute('data-a-target', 'unfollow-button');
							followBtn.setAttribute('aria-label', 'Unfollow');
						}
					})()`, nil),
				)

				// 4. Server-Side Persistence Verification (Wait 6 seconds to see if Twitch Apollo cache reverts)
				fmt.Printf("[Worker #%02d] ⏳ Verifying server-side persistence (waiting 6s for Twitch cache sync)...\n", id)
				time.Sleep(6 * time.Second)

				var verifyStatus struct {
					ButtonLabel string `json:"buttonLabel"`
					IsFollowing bool   `json:"isFollowing"`
				}
				_ = chromedp.Run(ctx, chromedp.Evaluate(`(() => {
					const btn = document.querySelector('button[data-a-target="follow-button"], button[data-a-target="unfollow-button"], button[aria-label="Following"], button[aria-label="Follow"]');
					const unfollow = document.querySelector('button[data-a-target="unfollow-button"], button[aria-label="Following"], button[aria-label="Unfollow"]');
					return {
						buttonLabel: btn ? (btn.innerText.trim() || btn.getAttribute('aria-label') || "") : "",
						isFollowing: unfollow !== null
					};
				})()`, &verifyStatus))

				if strings.EqualFold(verifyStatus.ButtonLabel, "follow") && !verifyStatus.IsFollowing {
					fmt.Printf("[Worker #%02d] ⚠️ [Server Rollback Detected]: Twitch server did NOT persist the follow (Button reverted to '%s').\n", id, verifyStatus.ButtonLabel)
				} else {
					fmt.Printf("[Worker #%02d] 💚 [Verified]: Follow state successfully persisted on Twitch server!\n", id)
				}
			}
		}
	}

	return nil
}

// simulateLiveChatting injects messages into the active stream interface with jitter
func simulateLiveChatting(ctx context.Context, id int, account *UserAccount, messages []string) {
	if account == nil || len(messages) == 0 {
		return
	}

	// Staggered Chat Distribution: initial randomized pause so workers do not all speak simultaneously
	initialStagger := time.Duration(15+rand.Intn(25)) * time.Second
	fmt.Printf("[Chatter #%02d] ⏳ Staggering initial chat for @%s (%v)...\n", id, account.Username, initialStagger)

	select {
	case <-ctx.Done():
		return
	case <-time.After(initialStagger):
	}

	chatInputSelector := `textarea[data-a-target="chat-input"], [data-a-target="chat-input"], [data-slate-editor="true"]`
	sendButtonSelector := `button[data-a-target="chat-send-button"]`

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg := messages[rand.Intn(len(messages))]
		fmt.Printf("[Chatter #%02d] 💬 Preparing to send message from @%s: %s\n", id, account.Username, msg)

		// Dismiss rules or modal if present
		_ = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const btn = document.querySelector('button[data-a-target="chat-rules-ok-button"], button[data-a-target="player-overlay-mature-accept"], button[data-a-target="consent-banner-accept"]');
					if (btn) btn.click();
				})()
			`, nil),
		)

		err := chromedp.Run(ctx,
			chromedp.WaitVisible(chatInputSelector, chromedp.ByQuery),
			chromedp.Focus(chatInputSelector, chromedp.ByQuery),
			chromedp.SendKeys(chatInputSelector, msg, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond),
			chromedp.Click(sendButtonSelector, chromedp.ByQuery),
		)

		if err != nil {
			// Fallback: send Enter key into input
			_ = chromedp.Run(ctx, chromedp.SendKeys(chatInputSelector, "\r", chromedp.ByQuery))
			fmt.Printf("[Chatter #%02d] Dispatch notice: %v\n", id, err)
		} else {
			fmt.Printf("[Chatter #%02d] 🎉 [Chat Sent] @%s: \"%s\"\n", id, account.Username, msg)
		}

		// Apply dynamic jitter (35 to 65 seconds) so accounts do not chat on a fixed interval
		jitterDelay := 35*time.Second + time.Duration(rand.Intn(30))*time.Second
		select {
		case <-ctx.Done():
			return
		case <-time.After(jitterDelay):
		}
	}
}

func runBrowserWorker(ctx context.Context, id int, targetURL string, proxy ProxyEndpoint, account *UserAccount, shouldFollow bool, shouldChat bool, wg *sync.WaitGroup, headless bool) {
	defer wg.Done()

	proxyURL := proxy.ChromeProxyURL
	displayName := proxy.DisplayName
	if displayName == "" {
		displayName = "Direct"
	}

	accLabel := "Anonymous Viewer"
	if account != nil {
		accLabel = fmt.Sprintf("@%s", account.Username)
	}
	fmt.Printf("[Worker #%02d] 🌐 Launching Chrome (Account: %s | Headless: %v) via: %s\n", id, accLabel, headless, displayName)

	execOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", headless),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("allow-running-insecure-content", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-infobars", true),
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

	// 1. Stealth Evasion & Auth Pre-Seed: Inject script on every new document to wipe out navigator.webdriver and pre-seed localStorage
	_ = chromedp.Run(browserCtx,
		chromedp.ActionFunc(func(actCtx context.Context) error {
			tokenVal := ""
			usernameVal := ""
			if account != nil {
				tokenVal = account.AuthToken
				usernameVal = account.Username
			}
			stealthJS := fmt.Sprintf(`
				Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
				window.chrome = { runtime: {} };
				Object.defineProperty(navigator, 'languages', { get: () => ['en-US', 'en'] });
				Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3, 4, 5] });
				try {
					if ('%s' !== '') {
						localStorage.setItem('auth-token', '%s');
						localStorage.setItem('twilight.oauth.token', '%s');
						localStorage.setItem('login', '%s');
					}
				} catch(e) {}
			`, tokenVal, tokenVal, tokenVal, usernameVal)
			_, err := page.AddScriptToEvaluateOnNewDocument(stealthJS).Do(actCtx)
			return err
		}),
	)

	// 2. Live Network Diagnostics: Listen to Twitch GQL responses
	chromedp.ListenTarget(browserCtx, func(ev interface{}) {
		if res, ok := ev.(*network.EventResponseReceived); ok {
			if strings.Contains(res.Response.URL, "gql.twitch.tv") {
				fmt.Printf("[Worker #%02d Network] 📡 Twitch GQL Response: HTTP %d (%s)\n", id, int(res.Response.Status), res.Response.StatusText)
			}
		}
	})

	// Execute account session loading, navigation, and follow action
	_ = performAccountActions(browserCtx, id, account, targetURL, shouldFollow)

	// If authenticated and chat is enabled, start live chat loop in background
	if account != nil && shouldChat {
		go simulateLiveChatting(browserCtx, id, account, chatMessageBank)
	}

	atomic.AddInt64(&activeWorkers, 1)
	defer atomic.AddInt64(&activeWorkers, -1)

	current := atomic.LoadInt64(&activeWorkers)
	fmt.Printf("[Worker #%02d] 💚 Chrome window open and permanently locked! Active Windows: %d\n", id, current)

	// Keep-Alive Loop: Window stays open permanently until Ctrl+C
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("[Worker #%02d] Stop signal received. Closing Chrome window.\n", id)
			return
		case <-ticker.C:
			var offlineMessage string
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
	channelFlag := flag.String("channel", "reallweston", "Target Twitch channel username")
	urlFlag := flag.String("url", "", "Custom target URL")
	workersFlag := flag.Int("workers", 1, "Number of concurrent browser instances (Default: 1)")
	accountFlag := flag.String("account", "", "Specific account username to use (Default: empty = rotate all available)")
	accountsDirFlag := flag.String("accounts-dir", "data/accounts", "Directory containing account JSON files")
	followFlag := flag.Bool("follow", true, "Execute follow action for authenticated accounts (Default: true)")
	chatFlag := flag.Bool("chat", true, "Execute live chat actions with staggered human timing (Default: true)")
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
	fmt.Println("⚡ GO CHROMEDP ENGINE (AUTHENTICATED SESSIONS, FOLLOW & CHAT)")
	fmt.Printf("   Target Stream:      %s\n", targetStream)
	fmt.Printf("   Active Workers:     %d windows\n", *workersFlag)
	fmt.Printf("   Display Mode:       %s\n", modeStr)
	fmt.Printf("   Auto-Follow Mode:   %v\n", *followFlag)
	fmt.Printf("   Live Chat Mode:     %v\n", *chatFlag)
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

	// Load authenticated accounts
	accDir := *accountsDirFlag
	if _, err := os.Stat(accDir); os.IsNotExist(err) {
		accDir = filepath.Join("..", "data", "accounts")
	}
	accounts := loadUserAccounts(accDir, *accountFlag)
	if len(accounts) > 0 {
		fmt.Printf("🔑 Loaded %d Authenticated Accounts from %s:\n", len(accounts), accDir)
		for _, a := range accounts {
			fmt.Printf("   • @%s (%d cookies)\n", a.Username, len(a.Cookies))
		}
		fmt.Println()
	} else {
		fmt.Println("ℹ️  No authenticated accounts found (Running in anonymous viewer mode).\n")
	}

	ctx, cancel := context.WithCancel(context.Background())
	if *totalDurationFlag > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*totalDurationFlag)*time.Minute)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup

	for w := 1; w <= *workersFlag; w++ {
		wg.Add(1)

		var acc *UserAccount
		if len(accounts) > 0 {
			a := accounts[(w-1)%len(accounts)]
			acc = &a
		}

		// Strict IP Lock-in: deterministically bind account to consistent proxy
		assignedProxy := getAccountProxy(acc, proxyPool)
		if acc != nil {
			fmt.Printf("[Security] 🔒 Strict IP Lock-in: @%s bound to %s\n", acc.Username, assignedProxy.DisplayName)
		}

		go runBrowserWorker(ctx, w, targetStream, assignedProxy, acc, *followFlag, *chatFlag, &wg, *headlessFlag)

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
