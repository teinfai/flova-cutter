package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed web/index.html
var page []byte

type instance struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

func (i instance) host() string    { return net.JoinHostPort("127.0.0.1", strconv.Itoa(i.Port)) }
func (i instance) pageURL() string { return "http://" + i.host() + "/?t=" + i.Token }

type settings struct {
	Target    string `json:"target"`
	Quality   string `json:"quality"`
	OutputDir string `json:"outputDir"`
}

type app struct {
	instance
	mu                           sync.Mutex
	data, cache, binDir, encoder string
	config                       settings
	tools                        toolState
	job                          *job
	lastPing                     time.Time
	closing                      bool
	ctx                          context.Context
	background                   sync.WaitGroup
}

func writeJSONFile(path string, v any) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".save-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = json.NewEncoder(f).Encode(v)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

func reopen(path string) bool {
	data, err := os.ReadFile(path)
	var i instance
	if err != nil || json.Unmarshal(data, &i) != nil || i.Port < 1 || i.Port > 65535 {
		return false
	}
	b, err := hex.DecodeString(i.Token)
	if err != nil || len(b) != 32 {
		return false
	}
	client := http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get("http://" + i.host() + "/api/ping?t=" + i.Token)
	if err != nil {
		return false
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	if err := openURL(i.pageURL()); err != nil {
		log.Print(err)
	}
	return true
}

func main() {
	if err := launch(); err != nil {
		log.Print(err)
	}
}

func launch() error {
	data, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	a := &app{data: filepath.Join(data, "FlovaCutter"), cache: filepath.Join(cache, "FlovaCutter")}
	a.binDir = filepath.Join(a.cache, "bin")
	if err := os.MkdirAll(a.data, 0700); err != nil {
		return err
	}
	lockPath := filepath.Join(a.data, "instance.json")
	if reopen(lockPath) {
		return nil
	}
	f, err := os.OpenFile(filepath.Join(a.data, "log.txt"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	for {
		acquired, err := lockInstance(f)
		if err != nil {
			return err
		}
		if acquired {
			break
		}
		if reopen(lockPath) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	log.SetOutput(f)
	defer log.SetOutput(os.Stderr)
	fail := func(err error) error { log.Print(err); return err }
	for _, dir := range []string{"work", "thumbs"} {
		if err := os.RemoveAll(filepath.Join(a.cache, dir)); err != nil {
			return fail(err)
		}
	}
	if err := os.MkdirAll(a.binDir, 0700); err != nil {
		return fail(err)
	}
	a.config = settings{"flova", "best", filepath.Join(home, "Downloads", "Flova Cutter")}
	cfg, err := os.ReadFile(filepath.Join(a.data, "config.json"))
	if err == nil {
		saved := a.config
		if err := json.Unmarshal(cfg, &saved); err != nil {
			log.Printf("invalid config.json: %v", err)
		} else if _, ok := findTarget(saved.Target); !ok || !validQuality(saved.Quality) || !filepath.IsAbs(saved.OutputDir) {
			log.Print("invalid config.json")
		} else {
			a.config = saved
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fail(err)
	}
	var secret [32]byte
	rand.Read(secret[:])
	a.Token = hex.EncodeToString(secret[:])
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fail(err)
	}
	defer listener.Close()
	a.Port = listener.Addr().(*net.TCPAddr).Port
	var cancel context.CancelFunc
	a.ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	a.lastPing = time.Now()
	server := &http.Server{Handler: a, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	if err := writeJSONFile(lockPath, a.instance); err != nil {
		return fail(err)
	}
	defer os.Remove(lockPath)
	a.background.Go(func() { a.prepareTools() })
	a.background.Go(func() {
		if err := openURL(a.pageURL()); err != nil {
			log.Print(err)
		}
	})
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case now := <-ticker.C:
				a.mu.Lock()
				a.closing = now.Sub(a.lastPing) > 10*time.Minute && !a.running()
				quit := a.closing
				a.mu.Unlock()
				if quit {
					server.Close()
					return
				}
			}
		}
	}()
	err = server.Serve(listener)
	cancel()
	a.background.Wait()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func validQuality(q string) bool { return q == "best" || q == "fast" }
func (a *app) running() bool     { return a.job != nil && a.job.cancel != nil }
func sameToken(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Print(err)
	}
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		reply(w, 400, failed)
		return false
	}
	return true
}

func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := a.host()
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	if r.Host != host || (r.Method == "POST" && len(r.Header.Values("Origin")) > 0 && (len(r.Header.Values("Origin")) != 1 || r.Header.Get("Origin") != "http://"+host)) {
		http.Error(w, "Forbidden", 403)
		return
	}
	path := r.URL.Path
	queryToken := sameToken(r.URL.Query().Get("t"), a.Token)
	if r.Method == "GET" && path == "/" {
		if !queryToken {
			http.Error(w, "Open Flova Cutter from its app icon.", 403)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(page)
		return
	}
	queryAllowed := path == "/api/ping" || strings.HasPrefix(path, "/files/")
	if !sameToken(r.Header.Get("X-Token"), a.Token) && !(queryAllowed && queryToken) {
		http.Error(w, "Forbidden", 403)
		return
	}
	switch {
	case path == "/api/ping" && (r.Method == "GET" || r.Method == "POST"):
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.closing {
			w.WriteHeader(503)
			return
		}
		a.lastPing = time.Now()
		if r.Method == "POST" {
			w.WriteHeader(204)
		} else {
			w.WriteHeader(200)
		}
	case path == "/api/state" && r.Method == "GET":
		a.mu.Lock()
		platform := "mac"
		if runtime.GOOS == "windows" {
			platform = "windows"
		}
		data, err := json.Marshal(map[string]any{"tools": a.tools, "settings": a.config, "os": platform, "job": a.job, "targets": targets})
		a.mu.Unlock()
		if err != nil {
			reply(w, 500, failed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	case path == "/api/start" && r.Method == "POST":
		a.start(w, r)
	case path == "/api/stop" && r.Method == "POST":
		a.mu.Lock()
		if a.running() {
			a.job.cancel()
		}
		a.mu.Unlock()
		w.WriteHeader(204)
	case path == "/api/settings" && r.Method == "POST":
		var body struct{ Target, Quality string }
		if !decode(w, r, &body) {
			return
		}
		if _, ok := findTarget(body.Target); !ok || !validQuality(body.Quality) {
			reply(w, 400, failed)
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		cfg := a.config
		cfg.Target, cfg.Quality = body.Target, body.Quality
		a.saveSettings(w, cfg)
	case path == "/api/choose-folder" && r.Method == "POST":
		dir, err := chooseFolder()
		if err != nil {
			log.Print(err)
			reply(w, 500, failed)
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		cfg := a.config
		if dir == "" {
			reply(w, 200, cfg)
			return
		}
		if !filepath.IsAbs(dir) {
			reply(w, 500, failed)
			return
		}
		cfg.OutputDir = filepath.Clean(dir)
		a.saveSettings(w, cfg)
	case path == "/api/open" && r.Method == "POST":
		var body struct{ File string }
		if r.ContentLength != 0 && !decode(w, r, &body) {
			return
		}
		a.open(w, body.File)
	case strings.HasPrefix(path, "/files/") && r.Method == "GET":
		a.serveFile(w, r, strings.TrimPrefix(path, "/files/"))
	default:
		http.NotFound(w, r)
	}
}

func (a *app) saveSettings(w http.ResponseWriter, cfg settings) {
	if err := writeJSONFile(filepath.Join(a.data, "config.json"), cfg); err != nil {
		log.Print(err)
		reply(w, 500, failed)
		return
	}
	a.config = cfg
	reply(w, 200, cfg)
}

func (a *app) filePath(name string, allowThumb bool) (string, string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.job == nil || a.job.Folder == "" {
		return "", "", false
	}
	if name == "" {
		return a.job.Folder, "", true
	}
	if !filepath.IsLocal(name) || strings.ContainsAny(name, `/\:`) || strings.Contains(name, "..") {
		return "", "", false
	}
	for _, f := range a.job.Files {
		if f.Name == name {
			return a.job.Folder, name, true
		}
		if allowThumb && f.Name+".jpg" == name {
			return a.job.thumbs, name, true
		}
	}
	return "", "", false
}

func (a *app) open(w http.ResponseWriter, name string) {
	dir, file, ok := a.filePath(name, false)
	if !ok {
		http.Error(w, "Forbidden", 403)
		return
	}
	var err error
	if file == "" {
		err = openFolder(dir)
	} else {
		err = revealFile(filepath.Join(dir, file))
	}
	if err != nil {
		log.Print(err)
		reply(w, 500, failed)
		return
	}
	w.WriteHeader(204)
}

func (a *app) serveFile(w http.ResponseWriter, r *http.Request, name string) {
	dir, file, ok := a.filePath(name, true)
	if !ok || file == "" {
		http.Error(w, "Forbidden", 403)
		return
	}
	f, err := os.OpenInRoot(dir, file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, file, info.ModTime(), f)
}

func native(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = procAttr()
	cmd.Stderr = log.Writer()
	log.Printf("%q", cmd.Args)
	return cmd
}

func clockTime(seconds float64) string {
	n := int(seconds)
	return fmt.Sprintf("%d:%02d:%02d", n/3600, n/60%60, n%60)
}
