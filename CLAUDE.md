# TippTrainer / SchreibTrainer

Zwei Varianten derselben Single-File-App (HTML + Inline-JS):

- `SchreibTrainer_4.0.html` – Standardversion (Quelle aller Änderungen). Sync wird von Nutzer:innen
  eingerichtet: eigener Server (Server-Programm / sync.php) oder Nextcloud.
- `SchreibTrainer_4.0_Nextcloud-PNS.html` – PNS-Version mit **absichtlich** fest im Code hinterlegten
  WebDAV-Zugangsdaten (`EMBEDDED_WEBDAV_CONFIG`) und eigenem `ADMIN_PASSWORD`. Nicht entfernen.

## Eingebettete Bausteine (keine CDN-Aufrufe)

Tailwind (vorkompiliert), Font Awesome, Schriften, html2pdf.js, JSZip und die Spielhalle stehen als Blöcke
`<!-- EINGEBETTET:name START … ENDE -->` in der Datei. Nie von Hand bearbeiten, sondern
`python3 werkzeuge/einbetten.py` ausführen (benötigt npx für Tailwind) – **nach jeder Änderung, die neue
Tailwind-Klassen verwendet**. Keine neuen externen `<script src>`, `<link>` oder `@import` einführen.
Externe Aufrufe zur Laufzeit gibt es nur für Übungsbilder (ARASAAC/Wikipedia); Offline-Exporte
(`lesson.offlineImages`) betten auch diese ein. Brand-Icons (`fab`) sind nicht eingebettet.

Die **Spielhalle** (`spielhalle.html`, 4 Arcade-Spiele) ist die Quelle des Blocks `spielhalle`: Änderungen dort, dann
`einbetten.py`. Die App lädt sie nur bei Bedarf per `srcdoc` in einen iframe (`openArcade()`); die Spielhalle meldet per
`postMessage` ihre Phasen (`Host` in `spielhalle.html`, `phase()` je Spiel). Münzregeln und 1-Minuten-Logik liegen in der
App (Abschnitt „SPIELE-BELOHNUNG“). Im Widget-/SCORM-Export wird der Block entfernt.

Reihenfolge nach Änderungen: `einbetten.py` → `pns_erzeugen.py` → ggf. `server_bauen.sh`.

## Regeln

- **Nur die Standardversion bearbeiten**, danach `python3 werkzeuge/pns_erzeugen.py` ausführen. Das Skript
  übernimmt Zugangsdaten und Admin-Passwort aus der bestehenden PNS-Datei und erzeugt sie neu. Danach dürfen
  sich die Dateien nur in diesen beiden Konstanten unterscheiden (`diff` prüfen).
- Zugangsdaten und Admin-Passwort dürfen nie in einen Widget-/SCORM-Export gelangen (`buildWidgetHtml()`).
- Nutzereingaben (Namen, Wörter, Eingaben) in HTML immer über `escHtml()` bzw. `escJsArg()` ausgeben.
- Cloud-Sync: Drei-Wege-Abgleich (`syncMergeSnapshots`) mit Versionsschutz beim Hochladen; neue Datenfelder,
  die Maps pro Lektion o.ä. sind, ggf. in die `deepKeys` aufnehmen (z.B. `coinEarned`/`coinSpent` der Münzen). Transport: `webDavGet`/`webDavPut`,
  Typ `webdav` (Nextcloud, ETag) oder `server` (Header `X-TippTrainer-Key/-Version/-If-Match`).
- Reihenfolgen nie über die Array-Reihenfolge speichern (wird beim Sync nicht abgeglichen), sondern über ein
  Feld pro Datensatz (`group.order`, `lesson.order`). Gruppen immer über `getOrderedGroups()` auflisten.

## Server (`Server/`)

- `sync.php` und das Go-Programm (`Server/quellcode/main.go`) implementieren dasselbe Sync-Protokoll – Änderungen
  immer an beiden vornehmen. Nur das Go-Programm liefert die App aus und ist „Passwortgeber“ (`admin.go`).
- **Admin-Passwort:** drei Betriebsarten in der App (`adminAuthMode()`): `fixed` (PNS: `ADMIN_PASSWORD`), `server`
  (Meta-Tag `tipptrainer-admin` vom Server-Programm, Hash `v1$runden$salz$hash`) und `local` (Einzel-HTML:
  `appSettings.adminHash`/`adminRecovery`, per Sync abgeglichen). Das Hash-Verfahren in `adminKdf()` (App) und
  `adminHash()` (`admin.go`) muss identisch bleiben; eigene SHA-256-Berechnung, weil WebCrypto über http:// fehlt.
  In der Standardversion ist `ADMIN_PASSWORD` leer (`""`) – `pns_erzeugen.py` setzt dort den PNS-Wert ein.
- Linux/Raspberry Pi: `Server/linux/` (Dienst-Datei, `installieren.sh`, `deinstallieren.sh`); Server-Parameter
  `--info`, `--passwort`, `--passwort-setzen`, `--passwort-neu`, `--hilfe` für Rechner ohne Bildschirm.
- Programme (Windows, macOS, Linux x64/arm64/arm32) neu bauen (bettet die aktuelle Standardversion als Rückfall ein):
  `sh werkzeuge/server_bauen.sh` (benötigt Go). Lehrkräfte-Anleitung: `Server/Anleitung.html`.

## Veröffentlichung

- Öffentliches Repository: https://github.com/Majort0m0/TippTrainer (Branch `main`). Die PNS-Datei, `_backup_vor_fixes/`,
  Server-Daten (`TippTrainer-Daten/`) und Build-Ergebnisse sind per `.gitignore` ausgeschlossen – vor jedem Commit
  mit `git grep --cached` sicherstellen, dass keine Zugangsdaten enthalten sind.
- Fertige Programme nicht ins Repository, sondern als GitHub-Release. Pakete mit `sh werkzeuge/release_pakete.sh`
  erzeugen (Ordner `dist/`, ignoriert): Server-Pakete für Windows/macOS (ZIP) und Linux/Raspberry Pi (tar.gz), `TippTrainer-Webspace-PHP.zip`, dazu App und
  `sync.php` einzeln. Hochladen: `gh release upload <tag> dist/* --clobber` (ZIP erhält das Ausführungsrecht der macOS-Datei).
- GitHub Pages (Branch `main`, Wurzel): https://majort0m0.github.io/TippTrainer/ – `index.html` leitet auf
  `SchreibTrainer_4.0.html` weiter. Jeder Push auf `main` aktualisiert die Online-Fassung automatisch.
