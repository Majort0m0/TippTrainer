// TippTrainer-Server: kleiner Schul-Server für den Tipp-Trainer.
//
// - liefert die App (SchreibTrainer-HTML) im lokalen Netzwerk aus
// - stellt unter /sync.php denselben Sync-Endpunkt bereit wie die PHP-Variante (sync.php)
// - speichert alle Daten in einem Ordner neben dem Programm (TippTrainer-Daten)
//
// Sync-Protokoll (identisch zu sync.php):
//
//	GET  /sync.php   Header X-TippTrainer-Key            → 200 + JSON + X-TippTrainer-Version | 404 (noch keine Daten)
//	PUT  /sync.php   Header X-TippTrainer-Key,
//	                 X-TippTrainer-If-Match: <version> | "*none*"
//	                                                    → 200 + X-TippTrainer-Version | 412 (anderes Gerät war schneller)
//	Falscher/fehlender Schlüssel → 401
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"rsc.io/qr"
)

//go:embed app.html
var embeddedApp []byte

const (
	dataDirName      = "TippTrainer-Daten"
	dataFileName     = "daten.json"
	maxBodyBytes     = 50 << 20 // 50 MB
	keepBackups      = 50
	keepDailyBackups = 30
)

type config struct {
	Key           string `json:"schluessel"`
	Port          int    `json:"port"`
	AdminPassword string `json:"adminPasswort,omitempty"` // Admin-Passwort der App (siehe admin.go)
	AdminSalt     string `json:"adminSalz,omitempty"`
}

var (
	baseDir string
	dataDir string
	cfg     config
	mu      sync.Mutex
	port    int
)

func main() {
	log.SetFlags(log.Ltime)
	// Hilfe ohne jede Nebenwirkung (legt keinen Datenordner an) – auch als Prüfung, ob das Programm hier läuft
	if len(os.Args) > 1 && isHelpArg(os.Args[1]) {
		fmt.Print(cliHelp)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		fatal("Programmpfad nicht ermittelbar: %v", err)
	}
	exe, _ = filepath.EvalSymlinks(exe)
	baseDir = filepath.Dir(exe)
	dataDir = filepath.Join(baseDir, dataDirName)
	if err := os.MkdirAll(filepath.Join(dataDir, "Sicherungen"), 0o700); err != nil {
		fatal("Datenordner kann nicht angelegt werden (%s): %v", dataDir, err)
	}
	loadOrCreateConfig()
	if handleCLI(os.Args[1:]) {
		return
	}
	newPassword := ensureAdminPassword()
	infoToken = randomString("abcdefghijklmnopqrstuvwxyz0123456789", 24)

	ln, err := listen(cfg.Port)
	if err != nil {
		fatal("Kein freier Port gefunden (ab %d): %v", cfg.Port, err)
	}
	port = ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/sync.php", handleSync)
	mux.HandleFunc("/sync", handleSync)
	mux.HandleFunc("/qr.png", handleQR)
	mux.HandleFunc("/info", handleInfo)
	mux.HandleFunc("/info/passwort", handleAdminPasswordForm)
	mux.HandleFunc("/info/login", handleInfoLogin)
	mux.HandleFunc("/info/abmelden", handleInfoLogout)
	mux.HandleFunc("/", handleApp)

	printBanner(newPassword)
	if os.Getenv("TIPPTRAINER_KEIN_BROWSER") == "" {
		go func() {
			time.Sleep(600 * time.Millisecond)
			openBrowser(fmt.Sprintf("http://localhost:%d/info", port))
		}()
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 15 * time.Second}
	if err := srv.Serve(ln); err != nil {
		fatal("Server beendet: %v", err)
	}
}

func fatal(format string, a ...any) {
	fmt.Printf("\nFEHLER: "+format+"\n\nZum Beenden Enter drücken …", a...)
	fmt.Scanln()
	os.Exit(1)
}

func listen(start int) (net.Listener, error) {
	var lastErr error
	for p := start; p < start+20; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err == nil {
			return ln, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func loadOrCreateConfig() {
	path := filepath.Join(dataDir, "einstellungen.json")
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	changed := false
	if len(cfg.Key) < 12 {
		buf := make([]byte, 15)
		_, _ = rand.Read(buf)
		cfg.Key = strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf))
		changed = true
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
		changed = true
	}
	if changed {
		b, _ := json.MarshalIndent(cfg, "", "  ")
		if err := os.WriteFile(path, b, 0o600); err != nil {
			fatal("Einstellungen können nicht gespeichert werden: %v", err)
		}
	}
}

// --- Sync-Endpunkt ---------------------------------------------------------------

func setCORS(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Content-Type, X-TippTrainer-Key, X-TippTrainer-If-Match")
	h.Set("Access-Control-Expose-Headers", "X-TippTrainer-Version")
	// Chrome: Zugriff aus dem öffentlichen Netz ins lokale Netz (Private Network Access)
	h.Set("Access-Control-Allow-Private-Network", "true")
	h.Set("Cache-Control", "no-store")
}

func versionOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:12])
}

func handleSync(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	key := r.Header.Get("X-TippTrainer-Key")
	if subtle.ConstantTimeCompare([]byte(key), []byte(cfg.Key)) != 1 {
		jsonError(w, http.StatusUnauthorized, "Schlüssel fehlt oder ist falsch")
		return
	}
	file := filepath.Join(dataDir, dataFileName)

	switch r.Method {
	case http.MethodGet:
		mu.Lock()
		b, err := os.ReadFile(file)
		mu.Unlock()
		if errors.Is(err, os.ErrNotExist) {
			jsonError(w, http.StatusNotFound, "Noch keine Daten vorhanden")
			return
		}
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "Daten nicht lesbar")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-TippTrainer-Version", versionOf(b))
		_, _ = w.Write(b)

	case http.MethodPut:
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
		if err != nil || len(body) > maxBodyBytes {
			jsonError(w, http.StatusRequestEntityTooLarge, "Daten zu groß")
			return
		}
		var probe map[string]any
		if json.Unmarshal(body, &probe) != nil {
			jsonError(w, http.StatusBadRequest, "Ungültige Daten")
			return
		}
		ifMatch := r.Header.Get("X-TippTrainer-If-Match")

		mu.Lock()
		defer mu.Unlock()
		current, err := os.ReadFile(file)
		exists := err == nil
		if ifMatch == "*none*" && exists {
			jsonError(w, http.StatusPreconditionFailed, "Daten wurden inzwischen angelegt")
			return
		}
		if ifMatch != "" && ifMatch != "*none*" && (!exists || ifMatch != versionOf(current)) {
			jsonError(w, http.StatusPreconditionFailed, "Daten wurden inzwischen geändert")
			return
		}
		if exists && !bytes.Equal(current, body) {
			backup(current)
		}
		tmp := file + ".tmp"
		if err := os.WriteFile(tmp, body, 0o600); err != nil {
			jsonError(w, http.StatusInternalServerError, "Speichern fehlgeschlagen")
			return
		}
		if err := os.Rename(tmp, file); err != nil {
			jsonError(w, http.StatusInternalServerError, "Speichern fehlgeschlagen")
			return
		}
		w.Header().Set("X-TippTrainer-Version", versionOf(body))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ok":true}`))
		log.Printf("Daten gespeichert (%d KB) von %s", len(body)/1024, clientIP(r))

	default:
		jsonError(w, http.StatusMethodNotAllowed, "Methode nicht erlaubt")
	}
}

// Ältere Fassungen aufheben (höchstens eine pro Minute, die letzten keepBackups Stück) und zusätzlich je Tag
// eine Sicherung (der Stand vom Vortag-Ende = vor dem ersten Speichern des Tages) für keepDailyBackups Tage.
func backup(current []byte) {
	dir := filepath.Join(dataDir, "Sicherungen")
	name := "daten-" + time.Now().Format("2006-01-02_15-04") + ".json"
	_ = os.WriteFile(filepath.Join(dir, name), current, 0o600)
	dailyBackup(dir, current)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var files []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "daten-") && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	for len(files) > keepBackups {
		_ = os.Remove(filepath.Join(dir, files[0]))
		files = files[1:]
	}
}

// Tagessicherung: "tag-JJJJ-MM-TT.json", einmal pro Tag; die letzten keepDailyBackups Tage bleiben erhalten.
func dailyBackup(dir string, current []byte) {
	name := "tag-" + time.Now().Format("2006-01-02") + ".json"
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		_ = os.WriteFile(filepath.Join(dir, name), current, 0o600)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var files []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "tag-") && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	for len(files) > keepDailyBackups {
		_ = os.Remove(filepath.Join(dir, files[0]))
		files = files[1:]
	}
}

func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"tipptrainerSync": 1, "fehler": msg})
}

func clientIP(r *http.Request) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

// --- App ausliefern ----------------------------------------------------------------

// Bevorzugt eine SchreibTrainer-HTML neben dem Programm (so lässt sich die App ohne neues
// Server-Programm aktualisieren), sonst die eingebaute Fassung.
func findAppFile() string {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return ""
	}
	var candidates []string
	for _, e := range entries {
		n := e.Name()
		low := strings.ToLower(n)
		if !e.IsDir() && strings.HasPrefix(low, "schreibtrainer") && strings.HasSuffix(low, ".html") && !strings.Contains(low, "pns") {
			candidates = append(candidates, n)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Strings(candidates)
	return filepath.Join(baseDir, candidates[len(candidates)-1])
}

func handleApp(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	page := embeddedApp
	if f := findAppFile(); f != "" {
		if b, err := os.ReadFile(f); err == nil {
			page = b
		}
	}
	_, _ = w.Write(markLocalServer(page))
}

// Kennzeichnet die ausgelieferte App als "lokal bereitgestellt": Die App blendet dann den
// Datenschutz-Hinweis beim ersten Start aus (siehe bootApp() in der HTML-Datei).
func markLocalServer(page []byte) []byte {
	marker := `<meta name="tipptrainer-lokaler-server" content="1">`
	// Admin-Passwort der App: nur als gesalzener Hash (siehe admin.go)
	if v := adminMetaValue(); v != "" {
		marker += "\n    " + `<meta name="tipptrainer-admin" content="` + v + `">`
	}
	i := bytes.Index(page, []byte("<head>"))
	if i < 0 {
		return page
	}
	i += len("<head>")
	out := make([]byte, 0, len(page)+len(marker)+1)
	out = append(out, page[:i]...)
	out = append(out, "\n    "+marker...)
	return append(out, page[i:]...)
}

// --- Info-Seite (wird beim Start auf dem Server-PC geöffnet) ---------------------------

func lanAddresses() []string {
	var out []string
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil || ipn.IP.IsLinkLocalUnicast() {
				continue
			}
			out = append(out, ipn.IP.String())
		}
	}
	sort.Strings(out)
	return out
}

// Geräte-Name im Netz ("<rechnername>.local", per mDNS/Bonjour/Avahi): bleibt gültig, auch wenn der Rechner in
// einem anderen Netz eine neue IP-Adresse bekommt. Leer, wenn der Rechnername dafür nicht taugt.
func localName() string {
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	h = strings.ToLower(strings.SplitN(h, ".", 2)[0])
	if h == "" || strings.ContainsAny(h, " _") {
		return ""
	}
	return h + ".local"
}

func nameConnectURL() string {
	n := localName()
	if n == "" {
		return ""
	}
	return fmt.Sprintf("http://%s:%d/#tt-server=%s", n, port, cfg.Key)
}

// Adressen der Info-Seite im Netzwerk (erst Gerätename, dann IP-Adressen)
func infoPageURLs() []string {
	var out []string
	if n := localName(); n != "" {
		out = append(out, fmt.Sprintf("http://%s:%d/info", n, port))
	}
	for _, ip := range lanAddresses() {
		out = append(out, fmt.Sprintf("http://%s:%d/info", ip, port))
	}
	return out
}

// Adresse inkl. Schlüssel im #-Teil: wird nicht an den Server übertragen; die App liest ihn
// beim Öffnen aus und richtet den Sync automatisch ein.
func connectURL(ip string) string {
	return fmt.Sprintf("http://%s:%d/#tt-server=%s", ip, port, cfg.Key)
}

func handleQR(w http.ResponseWriter, r *http.Request) {
	if !infoAuthorized(r) {
		http.NotFound(w, r)
		return
	}
	ip := r.URL.Query().Get("ip")
	code, err := qr.Encode(connectURL(ip), qr.M)
	if err != nil {
		http.Error(w, "QR-Fehler", 500)
		return
	}
	code.Scale = 8
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(code.PNG())
}

// Info-Seite und QR-Code (enthalten den Schlüssel): auf dem Server-PC selbst ohne Anmeldung, im Netz nur nach
// Eingabe des Admin-Passworts (siehe infoauth.go).
// isLocalRequest: Anfrage kommt vom Server-Rechner selbst.
func isLocalRequest(r *http.Request) bool {
	ip := net.ParseIP(clientIP(r))
	return ip != nil && ip.IsLoopback()
}

var infoTpl = template.Must(template.New("info").Parse(`<!doctype html>
<html lang="de"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>TippTrainer-Server</title>
<style>
 body{font-family:system-ui,-apple-system,Segoe UI,sans-serif;background:#f8fafc;color:#1e293b;margin:0;padding:24px}
 .card{background:#fff;border-radius:16px;box-shadow:0 2px 10px rgba(0,0,0,.08);padding:24px;max-width:820px;margin:0 auto 20px}
 h1{margin:0 0 4px;font-size:24px} .ok{color:#16a34a;font-weight:700}
 .addr{display:flex;gap:24px;align-items:center;flex-wrap:wrap;border-top:1px solid #e2e8f0;padding-top:16px;margin-top:16px}
 .addr img{width:220px;height:220px;image-rendering:pixelated;border:1px solid #e2e8f0;border-radius:8px}
 code{background:#f1f5f9;padding:2px 6px;border-radius:6px;font-size:15px;word-break:break-all}
 .big{font-size:20px;font-weight:700} li{margin:6px 0} .warn{color:#b45309}
 .link{display:flex;gap:8px;align-items:center;flex-wrap:wrap;margin:10px 0}
 .link a{color:#2563eb;font-family:ui-monospace,Menlo,Consolas,monospace;font-size:14px;word-break:break-all}
 button{border:1px solid #bfdbfe;background:#eff6ff;color:#1d4ed8;border-radius:8px;padding:6px 12px;font-weight:700;cursor:pointer;white-space:nowrap}
 button:hover{background:#dbeafe} .hint{color:#64748b;font-size:14px}
 .local{display:inline-block;margin-top:12px;background:#2563eb;color:#fff;text-decoration:none;padding:10px 16px;border-radius:10px;font-weight:700}
</style></head><body>
<div class="card">
 <h1>TippTrainer-Server</h1>
 <div class="ok">● läuft auf Port {{.Port}}</div>
 <p>Nur Lehrkräften zeigen – QR-Code und Link enthalten den Zugangsschlüssel.</p>
 {{if .Remote}}<form method="post" action="/info/abmelden" class="hint">Angemeldet über das Netzwerk (die Sitzung läuft nach etwa 30 Minuten ab). <button type="submit">Abmelden</button></form>{{end}}
 {{range .Addrs}}
 <div class="addr">
  <img src="/qr.png?ip={{.IP}}" alt="QR-Code">
  <div style="flex:1;min-width:260px">
   <div>Adresse für die Geräte:</div>
   <div class="big">http://{{.IP}}:{{$.Port}}</div>
   <p>iPad: QR-Code mit der Kamera scannen → in Safari öffnen.<br>Die App wird dadurch automatisch mit diesem Server verbunden.</p>
   <div class="hint">Oder diesen Link öffnen bzw. weitergeben (z.&nbsp;B. per Mail oder AirDrop an die iPads) – er macht dasselbe wie der QR-Code:</div>
   <div class="link"><a href="{{.Link}}" target="_blank">{{.Link}}</a> <button type="button" data-link="{{.Link}}" onclick="copyLink(this)">Link kopieren</button></div>
  </div>
 </div>
 {{else}}
 <p class="warn">Keine Netzwerkverbindung gefunden. Ist der PC mit dem (Schul-)Netz verbunden?</p>
 {{end}}
 {{if .NameLink}}
 <div style="margin-top:14px;border-top:1px solid #e2e8f0;padding-top:12px">
  <div><b>Mit dem Gerätenamen verbinden</b> – bleibt gültig, auch wenn der Server in ein anderes Netz kommt (solange dort Namen mit „.local“ erkannt werden, z.&nbsp;B. auf iPad und Mac):</div>
  <div class="link"><a href="{{.NameLink}}" target="_blank">{{.NameLink}}</a> <button type="button" data-link="{{.NameLink}}" onclick="copyLink(this)">Link kopieren</button></div>
 </div>
 {{end}}
</div>
<div class="card">
 <b>App auf {{if .Remote}}diesem Gerät{{else}}diesem PC{{end}} öffnen</b> – bereits mit dem Server verbunden:<br>
 <a class="local" href="{{.AppLink}}" target="_blank">App öffnen</a>
</div>
<div class="card">
 <b>Admin-Passwort der App</b>
 {{if .Saved}}<div class="ok">Gespeichert – gilt ab dem nächsten Laden der App.</div>{{end}}
 {{if .Failed}}<div class="warn">Das Passwort muss mindestens {{.MinLen}} Zeichen lang sein.</div>{{end}}
 {{if .AdminIsDefault}}<div class="warn">Es gilt noch das Standard-Passwort (im Quellcode des Programms öffentlich lesbar) – bitte ein eigenes festlegen.</div>{{end}}
 <p>Mit diesem Passwort schalten Lehrkräfte in der App den Admin-Modus ein. Es gilt für alle Geräte, die diese App vom Server laden.</p>
 <div class="big" style="font-size:22px">{{.AdminPassword}}</div>
 <form method="post" action="/info/passwort" style="margin-top:10px;display:flex;gap:8px;flex-wrap:wrap;align-items:center">
  <input type="hidden" name="token" value="{{.Token}}">
  <input type="text" name="passwort" placeholder="Neues Passwort" minlength="{{.MinLen}}" style="padding:8px;border:1px solid #cbd5e1;border-radius:8px">
  <button type="submit">Passwort ändern</button>
  <button type="submit" name="zufall" value="1">Zufälliges Passwort erzeugen</button>
 </form>
</div>
<div class="card">
 <b>Manuell verbinden</b> (z.&nbsp;B. am Lehrer-PC): App öffnen → Speicher-Symbol oben → „Eigener Server“ →<br>
 Adresse: <code>http://&lt;Adresse oben&gt;:{{.Port}}</code> &nbsp; Schlüssel: <code>{{.Key}}</code>
 <ul>
  <li>Daten liegen in: <code>{{.DataDir}}</code> (ältere Fassungen im Unterordner „Sicherungen“)</li>
  <li>Der PC muss eingeschaltet bleiben (Ruhezustand deaktivieren), solange synchronisiert werden soll. Die Geräte arbeiten sonst lokal weiter und gleichen später ab.</li>
  <li>Fragt die Firewall nach, Zugriff für <b>private Netzwerke</b> erlauben.</li>
  <li>Beenden: Konsolenfenster schließen (auf Linux/Raspberry Pi läuft der Server als Dienst: <code>sudo systemctl stop tipptrainer</code>).</li>
 </ul>
</div>
<script>
function copyLink(btn) {
  const text = btn.dataset.link;
  const done = () => { const t = btn.textContent; btn.textContent = 'Kopiert ✓'; setTimeout(() => btn.textContent = t, 1500); };
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text).then(done, () => fallback());
  } else { fallback(); }
  function fallback() {
    const ta = document.createElement('textarea'); ta.value = text; document.body.appendChild(ta); ta.select();
    try { document.execCommand('copy'); done(); } catch (e) { prompt('Link kopieren:', text); }
    ta.remove();
  }
}
</script>
</body></html>`))

func handleInfo(w http.ResponseWriter, r *http.Request) {
	if !infoAuthorized(r) {
		showInfoLogin(w, r)
		return
	}
	remote := !isLocalRequest(r)
	appLink := fmt.Sprintf("http://localhost:%d/#tt-server=%s", port, cfg.Key)
	if remote {
		appLink = fmt.Sprintf("http://%s/#tt-server=%s", r.Host, cfg.Key)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	type addr struct{ IP, Link string }
	var addrs []addr
	for _, ip := range lanAddresses() {
		addrs = append(addrs, addr{IP: ip, Link: connectURL(ip)})
	}
	q := r.URL.Query()
	_ = infoTpl.Execute(w, map[string]any{
		"AdminPassword": adminPassword(), "AdminIsDefault": adminPasswordIsDefault(), "Token": infoToken, "NameLink": nameConnectURL(),
		"Saved": q.Get("gespeichert") != "", "Failed": q.Get("fehler") != "", "MinLen": adminMinLength,
		"Port": port, "Addrs": addrs, "Key": cfg.Key, "DataDir": dataDir,
		"AppLink": appLink, "Remote": remote,
	})
}

func printBanner(newPassword bool) {
	fmt.Println("==============================================================")
	fmt.Println("  TippTrainer-Server läuft – dieses Fenster geöffnet lassen")
	fmt.Println("==============================================================")
	for _, ip := range lanAddresses() {
		fmt.Printf("  Adresse für die Geräte:  http://%s:%d\n", ip, port)
		fmt.Printf("  Link inkl. Schlüssel:    %s\n", connectURL(ip))
	}
	if u := nameConnectURL(); u != "" {
		fmt.Printf("  Link mit Gerätename:     %s\n", u)
	}
	fmt.Printf("  Schlüssel:               %s\n", cfg.Key)
	fmt.Printf("  Admin-Passwort der App:  %s\n", adminPassword())
	if newPassword {
		fmt.Println("  (Standard-Passwort – änderbar auf der Info-Seite oder mit:  --passwort-setzen NEU)")
	}
	fmt.Printf("  Daten:                   %s\n", dataDir)
	fmt.Printf("  QR-Codes & Hilfe:        http://localhost:%d/info\n", port)
	for _, u := range infoPageURLs() {
		fmt.Printf("  Info-Seite im Netz:      %s  (Admin-Passwort nötig)\n", u)
	}
	fmt.Println("==============================================================")
	if f := findAppFile(); f != "" {
		fmt.Printf("  App-Datei: %s\n", filepath.Base(f))
	} else {
		fmt.Println("  App-Datei: eingebaute Fassung")
	}
	fmt.Println()
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
