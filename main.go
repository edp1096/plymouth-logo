package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed web assets
var bundledAssets embed.FS
var assets resourceStore = bundledAssets

type payload struct {
	LogoPosition *position `json:"logo_position,omitempty"`
	SpinnerSize  int       `json:"spinner_size"`
	SpinnerItem  string    `json:"spinner_item"`
	Background   string    `json:"background"`
	Spinner      *position `json:"spinner,omitempty"`
	Image        string    `json:"image"`
	Size         int       `json:"size"`
}
type status struct {
	LogoPosition position `json:"logo_position"`
	SpinnerSize  int      `json:"spinner_size"`
	SpinnerItem  string   `json:"spinner_item"`
	Background   string   `json:"background"`
	Spinner      position `json:"spinner"`
	Busy         bool     `json:"busy"`
	OK           bool     `json:"ok"`
	Message      string   `json:"message"`
	Kernel       string   `json:"kernel"`
	Prepared     bool     `json:"prepared"`
	Size         int      `json:"size"`
	Applied      bool     `json:"applied"`
	Revision     uint64   `json:"revision"`
}
type app struct {
	preferencesPath string
	mu              sync.Mutex
	wg              sync.WaitGroup
	state           status
	token, host     string
	preview         []byte
	execute         func(string, string) ([]byte, error)
	watch           *windowWatch
}

func normalize(raw []byte, size int) ([]byte, error) { return resizePNG(raw, size, false) }
func resizePNG(raw []byte, size int, upscale bool) ([]byte, error) {
	if size < 32 || size > 1024 {
		return nil, fmt.Errorf("logo size must be 32–1024 pixels")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("select a valid PNG or JPEG image: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 20_000_000 {
		return nil, fmt.Errorf("image exceeds 20 megapixels")
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	w, h := cfg.Width, cfg.Height
	if w > size || h > size || upscale {
		if w >= h {
			h = max(1, h*size/w)
			w = size
		} else {
			w = max(1, w*size/h)
			h = size
		}
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	// Area averaging preserves alpha and avoids jagged downscaling.
	b := src.Bounds()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			x0, x1 := x*cfg.Width/w, max(x*cfg.Width/w+1, (x+1)*cfg.Width/w)
			y0, y1 := y*cfg.Height/h, max(y*cfg.Height/h+1, (y+1)*cfg.Height/h)
			var r, g, bl, a, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					rr, gg, bb, aa := src.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
					r += uint64(rr)
					g += uint64(gg)
					bl += uint64(bb)
					a += uint64(aa)
					n++
				}
			}
			i := dst.PixOffset(x, y)
			if a > 0 {
				dst.Pix[i] = uint8(r * 255 / a)
				dst.Pix[i+1] = uint8(g * 255 / a)
				dst.Pix[i+2] = uint8(bl * 255 / a)
			}
			dst.Pix[i+3] = uint8(a / n / 257)
		}
	}
	var out bytes.Buffer
	err = png.Encode(&out, dst)
	return out.Bytes(), err
}
func reply(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, code int, message string) {
	reply(w, code, map[string]string{"error": message})
}
func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' blob: data:; frame-ancestors 'none'; connect-src 'self'")
	if r.Host != a.host {
		fail(w, 403, "Invalid host")
		return
	}
	if r.Method == "GET" {
		files := map[string]string{"/": "web/index.html", "/app.js": "web/app.js", "/i18n.js": "web/i18n.js", "/style.css": "web/style.css", "/icon.png": "web/icon.png", "/presets.json": "web/presets.json", "/default.png": "assets/default.png"}
		if r.URL.Path == "/installed-spinner.png" || (strings.HasPrefix(r.URL.Path, "/spinner/") && strings.HasSuffix(r.URL.Path, ".png")) {
			var data []byte
			var err error
			if r.URL.Path == "/installed-spinner.png" {
				data, err = installedSpinnerPreview()
			} else {
				id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/spinner/"), ".png")
				data, err = customSpinnerPreview(id)
			}
			if err != nil {
				fail(w, 404, "Animation unavailable: "+err.Error())
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Write(data)
			return
		}
		if name, ok := files[r.URL.Path]; ok {
			data, err := assets.ReadFile(name)
			if err != nil {
				fail(w, 404, "Resource unavailable")
				return
			}
			mime := "text/html; charset=utf-8"
			if strings.HasSuffix(name, ".js") {
				mime = "text/javascript"
			}
			if strings.HasSuffix(name, ".json") {
				mime = "application/json"
			}
			if strings.HasSuffix(name, ".png") {
				mime = "image/png"
			}
			if strings.HasSuffix(name, ".css") {
				mime = "text/css"
			}
			w.Header().Set("Content-Type", mime)
			w.Write(data)
			return
		}
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-App-Token")), []byte(a.token)) != 1 {
		fail(w, 403, "Open the URL printed by the launcher")
		return
	}
	if r.URL.Path == "/api/preferences" {
		a.preferences(w, r)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/api/window" && a.watch != nil {
		a.watch.serve(w, r)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/api/spinners" {
		reply(w, 200, spinnerCatalog)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/api/status" {
		a.mu.Lock()
		s := a.state
		a.mu.Unlock()
		reply(w, 200, s)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/api/preview" {
		a.mu.Lock()
		data := append([]byte(nil), a.preview...)
		a.mu.Unlock()
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
		return
	}
	if r.Method != "POST" || (r.URL.Path != "/api/apply" && r.URL.Path != "/api/restore" && r.URL.Path != "/api/restore-original") {
		fail(w, 404, "Not found")
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+a.host {
		fail(w, 403, "Invalid origin")
		return
	}
	var p payload
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 17*1024*1024)).Decode(&p); err != nil {
		fail(w, 400, "Invalid or oversized request")
		return
	}
	action := strings.TrimPrefix(r.URL.Path, "/api/")
	if _, err := logoPosition(p.LogoPosition); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if _, err := spinnerSize(p.SpinnerItem, p.SpinnerSize); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if _, err := spinnerItem(p.SpinnerItem); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if _, err := backgroundColor(p.Background); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if _, err := spinnerPosition(p.Spinner); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if action == "apply" && (len(p.Image) == 0 || p.Size < 32 || p.Size > 1024) {
		fail(w, 400, "Select an image and a size between 32 and 1024 pixels")
		return
	}
	a.mu.Lock()
	if a.state.Busy {
		a.mu.Unlock()
		fail(w, 409, "An operation is already running")
		return
	}
	a.state.Busy = true
	a.state.OK = true
	a.state.Message = "Preparing…"
	a.wg.Add(1)
	a.mu.Unlock()
	go a.perform(action, p)
	reply(w, 202, map[string]bool{"accepted": true})
}
func (a *app) perform(action string, p payload) {
	defer a.wg.Done()
	var output []byte
	var err error
	defer func() {
		if err == nil {
			a.refreshPreview()
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		a.state.Busy = false
		a.state.OK = err == nil
		if err != nil {
			a.state.Message = fmt.Sprintf("%v\n%s", err, output)
		} else {
			a.state.Message = string(output)
		}
	}()
	path := ""
	if action == "apply" {
		var raw, converted []byte
		raw, err = base64.StdEncoding.DecodeString(p.Image)
		if err != nil {
			return
		}
		if len(raw) > 12*1024*1024 {
			err = fmt.Errorf("image exceeds 12 MB")
			return
		}
		converted, err = normalize(raw, p.Size)
		if err != nil {
			return
		}
		var f *os.File
		f, err = os.CreateTemp("", "plymouth-logo-*.png")
		if err != nil {
			return
		}
		path = f.Name()
		defer os.Remove(path)
		p.Image = base64.StdEncoding.EncodeToString(converted)
		err = json.NewEncoder(f).Encode(p)
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return
		}
	}
	a.mu.Lock()
	a.state.Message = "Waiting for administrator authentication, then processing…"
	a.mu.Unlock()
	output, err = a.execute(action, path)
	if len(output) > 6000 {
		output = output[len(output)-6000:]
	}
}
func main() {
	if len(os.Args) > 1 && os.Args[1] == "--helper" {
		if len(os.Args) < 4 {
			log.Fatal("missing helper resource path or action")
		}
		if err := useResources(os.Args[2]); err != nil {
			log.Fatal(err)
		}
		if err := helper(os.Args[3:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	port := flag.Int("port", 0, "Local HTTP port (0 selects a free port)")
	noBrowser := flag.Bool("no-browser", false, "Do not open a browser")
	dataDirFlag := flag.String("data-dir", defaultDataDir(), "Editable resource directory")
	installFlag := flag.Bool("install", false, "Install application-menu entry and executable")
	uninstallFlag := flag.Bool("uninstall", false, "Remove desktop integration, preserving resources")
	exportFlag := flag.Bool("export", false, "Export defaults if absent, print data directory, and exit")
	flag.Parse()
	if os.Geteuid() == 0 {
		log.Fatal("Run without sudo; apply/restore request administrator authentication")
	}
	dataDir, err := filepath.Abs(*dataDirFlag)
	if err != nil {
		log.Fatal(err)
	}
	if *uninstallFlag {
		if err = desktopIntegration(false, dataDir); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err = exportResources(bundledAssets, dataDir); err != nil {
		log.Fatal(err)
	}
	if err = useResources(dataDir); err != nil {
		log.Fatal(err)
	}
	if *exportFlag {
		fmt.Println(dataDir)
		return
	}
	if *installFlag {
		if err = desktopIntegration(true, dataDir); err != nil {
			log.Fatal(err)
		}
		return
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		log.Fatal(err)
	}
	initial, size, applied := installedPreview(configPath, themeDir+"/watermark.png")
	kernel, _ := exec.Command("uname", "-r").Output()
	executable, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		log.Fatal(err)
	}
	a := &app{preferencesPath: filepath.Join(dataDir, "preferences.json"), token: hex.EncodeToString(key), host: listener.Addr().String(), preview: initial, state: status{OK: true, Message: "Ready", Kernel: strings.TrimSpace(string(kernel)), Prepared: true, Size: size, Applied: applied, Revision: 1, Spinner: installedPosition(applied), Background: installedBackground(applied), SpinnerItem: installedSpinnerItem(applied), SpinnerSize: installedSpinnerSize(applied), LogoPosition: installedLogoPosition(applied)}}
	a.execute = func(action, path string) ([]byte, error) {
		args := []string{executable, "--helper", dataDir, action}
		if path != "" {
			args = append(args, path)
		}
		return exec.Command("/usr/bin/pkexec", args...).CombinedOutput()
	}
	server := &http.Server{Handler: a, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	url := "http://" + a.host + "/#" + a.token
	if *noBrowser {
		fmt.Printf("Plymouth Logo: %s\nCtrl+C to exit.\n", url)
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	requestStop := func() {
		select {
		case stop <- syscall.SIGTERM:
		default:
		}
	}
	if !*noBrowser {
		a.watch = &windowWatch{delay: time.Second, quit: requestStop}
		cmd, cleanup, err := appWindow(url)
		if err != nil {
			log.Fatal(err)
		}
		defer cleanup()
		cache, cacheErr := os.UserCacheDir()
		if cacheErr != nil {
			log.Fatal(cacheErr)
		}
		logFile, logErr := os.OpenFile(filepath.Join(cache, "plymouth-logo", "browser.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if logErr != nil {
			log.Fatal(logErr)
		}
		defer logFile.Close()
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err = cmd.Start(); err != nil {
			log.Fatalf("Open app window: %v", err)
		}
		go func() {
			if err := cmd.Wait(); err != nil {
				log.Printf("App window closed: %v", err)
			}
			requestStop()
		}()
	}
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()
	if err := server.Serve(listener); err != http.ErrServerClosed {
		log.Print(err)
	}
	a.wg.Wait()
}

// Keep io available to the helper's limited file reader through a common utility.
func readLimited(path string, n int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, n+1))
	if int64(len(data)) > n {
		return nil, fmt.Errorf("file is too large")
	}
	return data, err
}
