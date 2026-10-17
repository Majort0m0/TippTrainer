package main

// Zugang zur Info-Seite (/info) über das Netzwerk.
//
// Auf dem Server-Rechner selbst (localhost) ist die Info-Seite wie bisher ohne Anmeldung erreichbar.
// Von anderen Geräten im Netz (z. B. bei einem Raspberry Pi ohne Bildschirm) muss zuerst das Admin-Passwort
// eingegeben werden; danach gilt eine Sitzung (Cookie) für sessionTTL. Zu viele Fehlversuche sperren die
// Anmeldung von dieser Adresse für eine Weile. Die Seite zeigt Schlüssel, Link und QR-Code – deshalb der Schutz.

import (
	"crypto/subtle"
	"html/template"
	"net/http"
	"sync"
	"time"
)

const (
	infoCookie  = "tt_info"
	sessionTTL  = 30 * time.Minute
	maxFails    = 8
	failWindow  = 10 * time.Minute
	failPenalty = 700 * time.Millisecond
)

var (
	sessMu   sync.Mutex
	sessions = map[string]time.Time{}
	fails    = map[string][]time.Time{}
)

// Darf diese Anfrage die Info-Seite (inkl. QR-Code) sehen? Lokal immer, sonst mit gültiger Sitzung.
func infoAuthorized(r *http.Request) bool {
	if isLocalRequest(r) {
		return true
	}
	c, err := r.Cookie(infoCookie)
	if err != nil || c.Value == "" {
		return false
	}
	sessMu.Lock()
	defer sessMu.Unlock()
	exp, ok := sessions[c.Value]
	if !ok || time.Now().After(exp) {
		delete(sessions, c.Value)
		return false
	}
	sessions[c.Value] = time.Now().Add(sessionTTL) // Sitzung verlängert sich bei Nutzung
	return true
}

func recentFails(ip string) int {
	cut := time.Now().Add(-failWindow)
	kept := fails[ip][:0]
	for _, t := range fails[ip] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(fails, ip)
	} else {
		fails[ip] = kept
	}
	return len(kept)
}

func infoLocked(r *http.Request) bool {
	sessMu.Lock()
	defer sessMu.Unlock()
	return recentFails(clientIP(r)) >= maxFails
}

func handleInfoLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/info", http.StatusSeeOther)
		return
	}
	if infoLocked(r) {
		http.Redirect(w, r, "/info?gesperrt=1", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	pw := r.PostForm.Get("passwort")
	if subtle.ConstantTimeCompare([]byte(pw), []byte(adminPassword())) != 1 {
		sessMu.Lock()
		fails[clientIP(r)] = append(fails[clientIP(r)], time.Now())
		sessMu.Unlock()
		time.Sleep(failPenalty)
		http.Redirect(w, r, "/info?falsch=1", http.StatusSeeOther)
		return
	}
	token := randomString("abcdefghijklmnopqrstuvwxyz0123456789", 40)
	sessMu.Lock()
	now := time.Now()
	for k, exp := range sessions { // abgelaufene Sitzungen aufräumen
		if now.After(exp) {
			delete(sessions, k)
		}
	}
	sessions[token] = now.Add(sessionTTL)
	delete(fails, clientIP(r))
	sessMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: infoCookie, Value: token, Path: "/", MaxAge: int(sessionTTL.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/info", http.StatusSeeOther)
}

func handleInfoLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(infoCookie); err == nil {
		sessMu.Lock()
		delete(sessions, c.Value)
		sessMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: infoCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/info", http.StatusSeeOther)
}

var infoLoginTpl = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="de"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>TippTrainer-Server</title>
<style>
 body{font-family:system-ui,-apple-system,Segoe UI,sans-serif;background:#f8fafc;color:#1e293b;margin:0;padding:24px}
 .card{background:#fff;border-radius:16px;box-shadow:0 2px 10px rgba(0,0,0,.08);padding:24px;max-width:420px;margin:60px auto}
 h1{margin:0 0 4px;font-size:22px} .hint{color:#64748b;font-size:14px}
 input{width:100%;box-sizing:border-box;padding:12px;border:2px solid #cbd5e1;border-radius:10px;font-size:18px;margin:12px 0}
 button{width:100%;border:0;background:#2563eb;color:#fff;border-radius:10px;padding:12px;font-weight:700;font-size:16px;cursor:pointer}
 .warn{background:#fff7ed;color:#9a3412;border-radius:10px;padding:10px 14px;margin:12px 0}
</style></head><body>
<div class="card">
 <h1>TippTrainer-Server</h1>
 <p class="hint">Verbindungsdaten (Link und QR-Code für die Geräte) – bitte das <b>Admin-Passwort</b> eingeben.</p>
 {{if .Wrong}}<div class="warn">Das Passwort stimmt nicht.</div>{{end}}
 {{if .Locked}}<div class="warn">Zu viele Fehlversuche. Bitte in einigen Minuten erneut versuchen.</div>{{end}}
 <form method="post" action="/info/login">
  <input type="password" name="passwort" placeholder="Admin-Passwort" autofocus autocomplete="current-password">
  <button type="submit">Anmelden</button>
 </form>
</div></body></html>`))

func showInfoLogin(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = infoLoginTpl.Execute(w, map[string]any{"Wrong": q.Get("falsch") != "", "Locked": q.Get("gesperrt") != "" || infoLocked(r)})
}
