package main

// Admin-Passwort der App (nur im Server-Betrieb).
//
// Das Server-Programm ist "Passwortgeber": Beim ersten Start setzt es das Standard-Passwort
// (defaultAdminPassword, in einstellungen.json), zeigt es auf der Info-Seite und in der Kommandozeile an und liefert der App
// nur einen gesalzenen Hash mit (Meta-Tag "tipptrainer-admin"). Die App vergleicht das eingegebene
// Passwort mit diesem Hash — das Passwort selbst steht nie in der ausgelieferten Seite.
//
// Hash-Verfahren (muss exakt zur App passen, siehe adminKdf() in SchreibTrainer_4.0.html):
//   h = SHA-256(salz ":" passwort)
//   wiederhole <runden>-mal: h = SHA-256(h || salz || passwort)
//   Ergebnis hexadezimal; Meta-Wert: "v1$<runden>$<salz>$<hash>"
// (Ein eigenes Verfahren statt WebCrypto/PBKDF2, weil WebCrypto im Browser über http:// im Schulnetz
// nicht zur Verfügung steht.)

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"rsc.io/qr"
)

// Standard-Passwort bei neuen Installationen (bewusst einfach; die Lehrkraft kann es ändern).
// Steht nur hier im Server-Programm – die App bekommt ausschließlich einen gesalzenen Hash.
const defaultAdminPassword = "lernsachen"

const (
	adminKDFRounds   = 20000
	adminMinLength   = 4
	adminPasswordLen = 8
	// ohne leicht verwechselbare Zeichen (0/o, 1/l/i)
	adminAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"
)

var (
	adminMu     sync.Mutex
	adminCfgMod time.Time
	infoToken   string // Schutz des Formulars auf der Info-Seite gegen fremde Seiten im Browser
)

func settingsPath() string { return filepath.Join(dataDir, "einstellungen.json") }

func randomString(alphabet string, n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	out := make([]byte, n)
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out)
}

func adminHash(salt, password string, rounds int) string {
	first := sha256.Sum256([]byte(salt + ":" + password))
	sum := first[:]
	for i := 0; i < rounds; i++ {
		buf := make([]byte, 0, len(sum)+len(salt)+len(password))
		buf = append(buf, sum...)
		buf = append(buf, salt...)
		buf = append(buf, password...)
		next := sha256.Sum256(buf)
		sum = next[:]
	}
	return hex.EncodeToString(sum)
}

// Meta-Wert für die App (leer, wenn noch kein Passwort vorhanden ist)
func adminMetaValue() string {
	adminMu.Lock()
	defer adminMu.Unlock()
	reloadAdminLocked()
	if cfg.AdminPassword == "" || cfg.AdminSalt == "" {
		return ""
	}
	return fmt.Sprintf("v1$%d$%s$%s", adminKDFRounds, cfg.AdminSalt, adminHash(cfg.AdminSalt, cfg.AdminPassword, adminKDFRounds))
}

// true, wenn noch das Standard-Passwort gilt (Hinweis zum Ändern)
func adminPasswordIsDefault() bool { return adminPassword() == defaultAdminPassword }

func adminPassword() string {
	adminMu.Lock()
	defer adminMu.Unlock()
	reloadAdminLocked()
	return cfg.AdminPassword
}

// Liest die Admin-Felder neu, wenn einstellungen.json von außen geändert wurde (z. B. per Kommandozeile
// bei laufendem Server). Schlüssel und Port bleiben unverändert.
func reloadAdminLocked() {
	st, err := os.Stat(settingsPath())
	if err != nil || !st.ModTime().After(adminCfgMod) {
		return
	}
	adminCfgMod = st.ModTime()
	b, err := os.ReadFile(settingsPath())
	if err != nil {
		return
	}
	var fresh config
	if json.Unmarshal(b, &fresh) == nil {
		cfg.AdminPassword = fresh.AdminPassword
		cfg.AdminSalt = fresh.AdminSalt
	}
}

func saveConfigLocked() error {
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(settingsPath(), b, 0o600); err != nil {
		return err
	}
	if st, err := os.Stat(settingsPath()); err == nil {
		adminCfgMod = st.ModTime()
	}
	return nil
}

// Setzt das Passwort (neues Salz bei jeder Änderung)
func setAdminPassword(pw string) error {
	pw = strings.TrimSpace(pw)
	if len([]rune(pw)) < adminMinLength {
		return fmt.Errorf("Das Passwort muss mindestens %d Zeichen lang sein.", adminMinLength)
	}
	adminMu.Lock()
	defer adminMu.Unlock()
	reloadAdminLocked()
	cfg.AdminPassword = pw
	cfg.AdminSalt = randomString("0123456789abcdef", 16)
	return saveConfigLocked()
}

// Beim Start: fehlendes Passwort erzeugen. Gibt zurück, ob ein neues erzeugt wurde.
func ensureAdminPassword() bool {
	adminMu.Lock()
	defer adminMu.Unlock()
	reloadAdminLocked()
	if cfg.AdminPassword != "" && cfg.AdminSalt != "" {
		return false
	}
	if cfg.AdminPassword == "" {
		cfg.AdminPassword = defaultAdminPassword
	}
	cfg.AdminSalt = randomString("0123456789abcdef", 16)
	if err := saveConfigLocked(); err != nil {
		fatal("Einstellungen können nicht gespeichert werden: %v", err)
	}
	return true
}

// --- Info-Seite: Passwort ändern -------------------------------------------------------

func handleAdminPasswordForm(w http.ResponseWriter, r *http.Request) {
	if !isLocalRequest(r) || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Formular unlesbar", http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("token")), []byte(infoToken)) != 1 {
		http.Error(w, "Ungültige Anfrage – bitte die Info-Seite neu laden.", http.StatusForbidden)
		return
	}
	pw := r.PostForm.Get("passwort")
	if r.PostForm.Get("zufall") != "" {
		pw = randomString(adminAlphabet, adminPasswordLen)
	}
	if err := setAdminPassword(pw); err != nil {
		http.Redirect(w, r, "/info?fehler=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/info?gespeichert=1", http.StatusSeeOther)
}

// --- Kommandozeile ---------------------------------------------------------------------

const cliHelp = `TippTrainer-Server

Ohne Parameter startet der Server. Für Rechner ohne Bildschirm (z. B. Raspberry Pi):

  --info                  Adressen, Verbindungs-Link, QR-Code und Admin-Passwort anzeigen
  --passwort              Admin-Passwort der App anzeigen
  --passwort-setzen NEU   Admin-Passwort der App ändern (mindestens 4 Zeichen)
  --passwort-neu          Zufälliges neues Admin-Passwort erzeugen
  --hilfe                 Diese Hilfe

Änderungen gelten sofort, auch wenn der Server bereits läuft.
`

func isHelpArg(a string) bool {
	return a == "--hilfe" || a == "-h" || a == "--help" || a == "-?"
}

// Verarbeitet Kommandozeilen-Parameter. Gibt true zurück, wenn der Server danach nicht starten soll.
func handleCLI(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "--passwort":
		ensureAdminPassword()
		fmt.Println(adminPassword())
	case "--passwort-setzen":
		if len(args) < 2 {
			fmt.Println("Bitte das neue Passwort angeben:  --passwort-setzen NEUESPASSWORT")
			os.Exit(2)
		}
		ensureAdminPassword()
		if err := setAdminPassword(args[1]); err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		fmt.Println("Admin-Passwort geändert.")
	case "--passwort-neu":
		ensureAdminPassword()
		pw := randomString(adminAlphabet, adminPasswordLen)
		if err := setAdminPassword(pw); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println(pw)
	case "--info":
		ensureAdminPassword()
		port = cfg.Port
		printInfoCLI()
	default:
		fmt.Printf("Unbekannter Parameter: %s\n\n%s", args[0], cliHelp)
		os.Exit(2)
	}
	return true
}

func printInfoCLI() {
	fmt.Println("TippTrainer-Server – Verbindungsdaten")
	fmt.Println("--------------------------------------------------------------")
	ips := lanAddresses()
	if len(ips) == 0 {
		fmt.Println("Keine Netzwerkverbindung gefunden.")
	}
	for i, ip := range ips {
		fmt.Printf("Adresse für die Geräte:  http://%s:%d\n", ip, port)
		fmt.Printf("Link inkl. Schlüssel:    %s\n", connectURL(ip))
		if i == 0 {
			fmt.Println("\nQR-Code (iPad: mit der Kamera scannen):")
			printQRText(connectURL(ip))
		}
	}
	if u := nameConnectURL(); u != "" {
		fmt.Printf("Link mit Gerätename:     %s\n", u)
		fmt.Println("                         (bleibt in jedem Netz gültig, in dem Namen mit „.local“ erkannt werden)")
	}
	fmt.Printf("Schlüssel:               %s\n", cfg.Key)
	fmt.Printf("Admin-Passwort (App):    %s\n", adminPassword())
	fmt.Println("(Hinweis: Läuft der Server auf einem anderen Port, weil dieser belegt war, steht er im Protokoll.)")
}

// QR-Code im Terminal (zwei Bildpunkte pro Zeichen, feste Farben – unabhängig vom Terminal-Design)
func printQRText(content string) {
	code, err := qr.Encode(content, qr.M)
	if err != nil {
		fmt.Println("(QR-Code nicht darstellbar)")
		return
	}
	const quiet = 2
	size := code.Size
	dark := func(x, y int) bool {
		if x < 0 || y < 0 || x >= size || y >= size {
			return false
		}
		return code.Black(x, y)
	}
	color := func(black bool) string {
		if black {
			return "16"
		}
		return "231"
	}
	for y := -quiet; y < size+quiet; y += 2 {
		var sb strings.Builder
		for x := -quiet; x < size+quiet; x++ {
			sb.WriteString(fmt.Sprintf("\x1b[38;5;%sm\x1b[48;5;%sm▀", color(dark(x, y)), color(dark(x, y+1))))
		}
		sb.WriteString("\x1b[0m")
		fmt.Println(sb.String())
	}
}
