package finance

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed ui/*
var web embed.FS

func UI() fs.FS { f, _ := fs.Sub(web, "ui"); return f }

type consent struct {
	User, Session string
	Expires       time.Time
	Used          bool
}
type App struct {
	h                    *apphost.Host
	engine, cap, runtime string
	mu                   sync.Mutex
	states               map[string]consent
	ready                bool
}

func New(h *apphost.Host, digest string) (*App, error) {
	root, e := unpack(h.Config.Data, digest)
	if e != nil {
		return nil, e
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return nil, e
	}
	addr := l.Addr().String()
	l.Close()
	a := &App{h: h, engine: "http://" + addr, cap: apphost.ID() + apphost.ID(), runtime: root, states: map[string]consent{}}
	h.Mux.HandleFunc("/ledger/", a.engineHandler)
	h.Mux.HandleFunc("GET /ledger", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, h.Base+"/ledger/", 303) })
	h.Mux.HandleFunc("GET /api/finance/status", h.Require(a.status))
	h.Mux.HandleFunc("GET /api/finance/providers", h.Require(func(w http.ResponseWriter, r *http.Request) {
		apphost.JSON(w, 200, map[string]any{"providers": []map[string]any{{"id": "enable-banking", "name": "Enable Banking", "experimental": true, "setup": "Set up Enable Banking in the budget's Bank Sync settings. UK directory is retrieved with your provider application.", "country": "GB"}, {"id": "lunchflow-simplefin", "name": "Lunch Flow via SimpleFIN", "setup": "Requires your own supported Lunch Flow subscription and one-use setup token."}}, "manualImport": "CSV, OFX and QIF in the native account Import menu", "headlessSync": false})
	}))
	h.Mux.HandleFunc("POST /api/finance/backups/encrypt", h.Owner(a.encryptBackup))
	h.Mux.HandleFunc("POST /api/finance/backups/decrypt", h.Owner(a.decryptBackup))
	return a, nil
}
func (a *App) OpenBrowser(link string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", link).Start()
}
func (a *App) Run(ctx context.Context) error {
	data := filepath.Join(a.h.Config.Data, "actual")
	if e := os.MkdirAll(data, 0700); e != nil {
		return e
	}
	port := strings.Split(a.engine, ":")[2]
	// Only this minimum environment crosses into the native engine. Provider debug
	// output and upstream HTTP body/URL logging never enter the Workspace log.
	env := []string{"NODE_ENV=production", "ACTUAL_HOSTNAME=127.0.0.1", "ACTUAL_PORT=" + port, "ACTUAL_DATA_DIR=" + data, "ACTUAL_CORS_PROXY_ENABLED=false", "MYFINANCE_ENGINE_CAPABILITY=" + a.cap}
	done := make(chan error, 1)
	go func() {
		done <- apphost.Service(ctx, filepath.Join(a.runtime, "node.exe"), a.runtime, []string{filepath.Join(a.runtime, "node_modules/@actual-app/sync-server/build/app.js")}, env, io.Discard)
	}()
	client := http.Client{Timeout: time.Second}
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(60 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case e := <-done:
			if ctx.Err() != nil {
				return nil
			}
			if e == nil {
				e = errors.New("finance engine stopped")
			}
			return e
		case <-ctx.Done():
			<-done
			return nil
		case <-deadline.C:
			return errors.New("Actual engine startup timed out")
		case <-ticker.C:
			req, _ := http.NewRequest("GET", a.engine+"/health", nil)
			req.Header.Set("X-Finance-Capability", a.cap)
			req.Header.Set("X-Finance-Identity", strings.Repeat("0", 64))
			resp, e := client.Do(req)
			if e == nil {
				resp.Body.Close()
				if resp.StatusCode == 200 {
					a.mu.Lock()
					a.ready = true
					a.mu.Unlock()
					select {
					case e = <-done:
						if ctx.Err() != nil {
							return nil
						}
						return e
					case <-ctx.Done():
						<-done
						return nil
					}
				}
			}
		}
	}
}
func (a *App) status(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	ready := a.ready
	a.mu.Unlock()
	apphost.JSON(w, 200, map[string]any{"actual": "26.10.0", "engineReady": ready, "currency": "GBP", "headlessSync": "Unavailable with this enforced OneAccount/E2EE configuration. Use native Sync while your budget is open.", "backups": "Encrypted native budget exports. Provider credentials are excluded. Scheduled backup is not enabled.", "driveConfigured": os.Getenv("MYFINANCE_BACKUP_DIR") != ""})
}
func originOf(s string) string {
	u, e := url.Parse(s)
	if e != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
func (a *App) engineHandler(w http.ResponseWriter, r *http.Request) {
	// apphost's native Require verifies the cookie and expiry. Engine requests use
	// exact browser Origin checks instead of modifying the engine's worker protocol.
	method := r.Method
	if method != "GET" && method != "HEAD" && r.Header.Get("Origin") != originOf(a.h.Config.Origin) {
		apphost.Problem(w, 403, "Open the configured Finance address before changing a budget")
		return
	}
	clone := r.Clone(r.Context())
	clone.Method = "GET"
	a.h.Require(func(w http.ResponseWriter, authenticated *http.Request) {
		authenticated.Method = method
		a.proxy(w, authenticated)
	})(w, clone)
}
func (a *App) proxy(w http.ResponseWriter, r *http.Request) {
	s := apphost.Current(r)
	p := strings.TrimPrefix(r.URL.Path, "/ledger")
	if p == "/sw.js" {
		http.Error(w, "Service worker unavailable on the shared Workspace origin", 404)
		return
	}
	if strings.Contains(p, "..") || strings.Contains(p, "\\") {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(p, "/enablebanking/") || strings.HasPrefix(p, "/simplefin/") || strings.HasPrefix(p, "/secret/") {
		if s.User.Role != "owner" {
			apphost.Problem(w, 403, "Only the household owner manages bank access")
			return
		}
	}
	if p == "/finance-bridge.js" {
		w.Header().Set("Content-Type", "text/javascript")
		w.Header().Set("Content-Security-Policy", "script-src 'self'")
		io.WriteString(w, bridgeJS(a.h.Base))
		return
	}
	if p == "/enablebanking/complete-auth" {
		apphost.Problem(w, 403, "Complete consent using the registered bank callback")
		return
	}
	if p == "/enablebanking/auth_callback" {
		if !a.checkState(r.URL.Query().Get("state"), s, true) {
			apphost.Problem(w, 400, "Bank consent expired, belongs to another session, or has already been used. Start again.")
			return
		}
	}
	if p == "/enablebanking/start-auth" || p == "/enablebanking/poll-auth" || p == "/enablebanking/aspsps" {
		body, e := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
		if e != nil || len(body) > 1<<20 {
			apphost.Problem(w, 400, "Request too large")
			return
		}
		var input map[string]any
		if json.Unmarshal(body, &input) != nil {
			apphost.Problem(w, 400, "Invalid provider request")
			return
		}
		if p == "/enablebanking/start-auth" {
			callback := a.h.Config.Origin + "/ledger/enablebanking/auth_callback"
			if input["redirectUrl"] != callback || !strings.HasPrefix(callback, "https://") {
				apphost.Problem(w, 400, "Use the exact configured HTTPS bank callback")
				return
			}
			if inst, ok := input["aspsp"].(map[string]any); !ok || inst["country"] != "GB" {
				apphost.Problem(w, 400, "Choose an available UK bank")
				return
			}
			input["psuType"] = "personal"
		}
		if p == "/enablebanking/aspsps" {
			input["country"] = "GB"
		}
		if p == "/enablebanking/poll-auth" {
			state, _ := input["state"].(string)
			if !a.checkState(state, s, false) {
				apphost.Problem(w, 400, "Consent session expired")
				return
			}
		}
		body, _ = json.Marshal(input)
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
	}
	target, _ := url.Parse(a.engine)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = p
		req.Host = target.Host
		req.Header.Del("Cookie")
		req.Header.Del("Authorization")
		req.Header.Del("X-Actual-Password")
		req.Header.Del("X-Actual-Token")
		req.Header.Del("X-Forwarded-For")
		req.Header.Del("X-Forwarded-Host")
		req.Header.Set("X-Finance-Capability", a.cap)
		req.Header.Set("X-Finance-Identity", s.User.ID)
		req.Header.Set("X-Finance-Owner", map[bool]string{true: "1", false: "0"}[s.User.Role == "owner"])
		req.Header.Del("Accept-Encoding")
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("Content-Security-Policy")
		resp.Header.Del("Access-Control-Allow-Origin")
		resp.Header.Del("Access-Control-Allow-Credentials")
		resp.Header.Del("Set-Cookie")
		resp.Header.Set("Cache-Control", "no-store")
		// Actual's native SQLite worker requires WASM and same-origin isolation.
		resp.Header.Set("Content-Security-Policy", "default-src 'self' blob:; script-src 'self' 'unsafe-eval' blob:; style-src 'self' 'unsafe-inline'; img-src 'self' blob: data:; font-src 'self' data:; connect-src 'self'; worker-src 'self' blob:; object-src 'none'; base-uri 'self'; frame-ancestors 'self'")
		resp.Header.Set("Cross-Origin-Opener-Policy", "same-origin")
		resp.Header.Set("Cross-Origin-Embedder-Policy", "require-corp")
		ct := resp.Header.Get("Content-Type")
		if p == "/enablebanking/start-auth" || strings.Contains(ct, "text/html") || strings.Contains(ct, "javascript") || strings.HasSuffix(p, ".css") {
			body, e := io.ReadAll(io.LimitReader(resp.Body, 40<<20))
			resp.Body.Close()
			if e != nil {
				return e
			}
			if p == "/enablebanking/start-auth" {
				var result struct {
					Status string
					Data   struct{ State string }
				}
				if json.Unmarshal(body, &result) == nil && result.Status == "ok" && result.Data.State != "" {
					a.addState(result.Data.State, s)
				}
			}
			if strings.Contains(ct, "text/html") || strings.Contains(ct, "javascript") || strings.HasSuffix(p, ".css") {
				body = []byte(rebase(string(body), a.h.Base+"/ledger"))
			}
			if strings.Contains(ct, "text/html") && strings.Contains(string(body), "<head>") {
				body = bytes.Replace(body, []byte("<head>"), []byte("<head><script src=\""+a.h.Base+"/ledger/finance-bridge.js\"></script>"), 1)
			}
			resp.Body = io.NopCloser(bytes.NewReader(body))
			resp.ContentLength = int64(len(body))
			resp.Header.Del("Content-Length")
			resp.Header.Del("Etag")
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		apphost.Problem(w, 503, "The budgeting engine is starting or needs a restart. Your saved data is retained.")
	}
	proxy.ServeHTTP(w, r)
}
func (a *App) addState(state string, s apphost.Session) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for k, v := range a.states {
		if time.Now().After(v.Expires) {
			delete(a.states, k)
		}
	}
	if len(a.states) >= 128 {
		return
	}
	a.states[state] = consent{s.User.ID, s.CSRF, time.Now().Add(5 * time.Minute), false}
}
func (a *App) checkState(state string, s apphost.Session, consume bool) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	v, ok := a.states[state]
	if !ok || v.User != s.User.ID || v.Session != s.CSRF || time.Now().After(v.Expires) || consume && v.Used {
		return false
	}
	if consume {
		v.Used = true
		a.states[state] = v
	}
	return true
}
func rebase(body, base string) string {
	for _, p := range []string{"/static/", "/kcab/", "/data/", "/sql-wasm.wasm", "/favicon", "/apple-touch-icon", "/site.webmanifest", "/manifest.webmanifest"} {
		body = strings.ReplaceAll(body, p, base+p)
	}
	body = strings.ReplaceAll(body, "let serverURL=window.location.origin,", "let serverURL=window.location.origin+"+strconvQuote(base)+",")
	body = strings.ReplaceAll(body, "setUrl(window.location.origin)", "setUrl(window.location.origin+"+strconvQuote(base)+")")
	body = strings.ReplaceAll(body, "BrowserRouter,{children:", "BrowserRouter,{basename:"+strconvQuote(base)+",children:")
	body = strings.ReplaceAll(body, "${window.location.origin}/enablebanking/auth_callback", "${window.location.origin}"+base+"/enablebanking/auth_callback")
	return body
}
func strconvQuote(s string) string { b, _ := json.Marshal(s); return string(b) }
func bridgeJS(base string) string  { return `document.title='My Finance · Actual Budget';` }

func unpack(data, digest string) (string, error) {
	exe, e := os.Executable()
	if e != nil {
		return "", e
	}
	archive := filepath.Join(filepath.Dir(exe), "finance-runtime.zip")
	b, e := os.ReadFile(archive)
	if e != nil {
		return "", errors.New("Keep finance-runtime.zip beside MyFinance.exe")
	}
	sum := sha256.Sum256(b)
	if digest != "development" && hex.EncodeToString(sum[:]) != digest {
		return "", errors.New("Finance runtime checksum mismatch")
	}
	root := filepath.Join(data, "engine", hex.EncodeToString(sum[:8]))
	if e=safeAncestors(root);e!=nil{return "",e}
	if _, e = os.Stat(filepath.Join(root, ".complete")); e == nil {
		return root, nil
	}
	z, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		return "", e
	}
	if e = os.MkdirAll(root, 0700); e != nil {
		return "", e
	}
	for _, f := range z.File {
		name := filepath.FromSlash(f.Name)
		if name == "." || filepath.IsAbs(name) || strings.Contains(name, ":") || f.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("Unsafe runtime member")
		}
		target := filepath.Join(root, name)
		if !strings.HasPrefix(target, root+string(os.PathSeparator)) {
			return "", errors.New("Runtime path escaped")
		}
		if f.FileInfo().IsDir() {
			continue
		}
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			return "", e
		}
		in, e := f.Open()
		if e != nil {
			return "", e
		}
		out, e := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if e != nil {
			in.Close()
			return "", e
		}
		_, e = io.Copy(out, in)
		in.Close()
		out.Close()
		if e != nil {
			return "", e
		}
	}
	e = os.WriteFile(filepath.Join(root, ".complete"), []byte(digest), 0600)
	return root, e
}
