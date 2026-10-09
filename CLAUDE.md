# TippTrainer / SchreibTrainer

Zwei Varianten derselben Single-File-App (HTML + Inline-JS):

- `SchreibTrainer_4.0.html` – Standardversion (Quelle aller Änderungen). Sync wird von Nutzer:innen
  eingerichtet: eigener Server (Server-Programm / sync.php) oder Nextcloud.
- `SchreibTrainer_4.0_Nextcloud-PNS.html` – PNS-Version mit **absichtlich** fest im Code hinterlegten
  WebDAV-Zugangsdaten (`EMBEDDED_WEBDAV_CONFIG`) und eigenem `ADMIN_PASSWORD`. Nicht entfernen.

## Eingebettete Bausteine (keine CDN-Aufrufe)

Tailwind (vorkompiliert), Font Awesome, Schriften, html2pdf.js und JSZip stehen als Blöcke
`<!-- EINGEBETTET:name START … ENDE -->` in der Datei. Nie von Hand bearbeiten, sondern
`python3 werkzeuge/einbetten.py` ausführen (benötigt npx für Tailwind) – **nach jeder Änderung, die neue
Tailwind-Klassen verwendet**. Keine neuen externen `<script src>`, `<link>` oder `@import` einführen.
Externe Aufrufe zur Laufzeit gibt es nur für Übungsbilder (ARASAAC/Wikipedia); Offline-Exporte
(`lesson.offlineImages`) betten auch diese ein. Brand-Icons (`fab`) sind nicht eingebettet.

Reihenfolge nach Änderungen: `einbetten.py` → `pns_erzeugen.py` → ggf. `server_bauen.sh`.

## Regeln

- **Nur die Standardversion bearbeiten**, danach `python3 werkzeuge/pns_erzeugen.py` ausführen. Das Skript
  übernimmt Zugangsdaten und Admin-Passwort aus der bestehenden PNS-Datei und erzeugt sie neu. Danach dürfen
  sich die Dateien nur in diesen beiden Konstanten unterscheiden (`diff` prüfen).
- Zugangsdaten und Admin-Passwort dürfen nie in einen Widget-/SCORM-Export gelangen (`buildWidgetHtml()`).
- Nutzereingaben (Namen, Wörter, Eingaben) in HTML immer über `escHtml()` bzw. `escJsArg()` ausgeben.
- Cloud-Sync: Drei-Wege-Abgleich (`syncMergeSnapshots`) mit Versionsschutz beim Hochladen; neue Datenfelder,
  die Maps pro Lektion o.ä. sind, ggf. in die `deepKeys` aufnehmen. Transport: `webDavGet`/`webDavPut`,
  Typ `webdav` (Nextcloud, ETag) oder `server` (Header `X-TippTrainer-Key/-Version/-If-Match`).
- Reihenfolgen nie über die Array-Reihenfolge speichern (wird beim Sync nicht abgeglichen), sondern über ein
  Feld pro Datensatz (`group.order`, `lesson.order`). Gruppen immer über `getOrderedGroups()` auflisten.

## Server (`Server/`)

- `sync.php` und das Go-Programm (`Server/quellcode/main.go`) implementieren dasselbe Sync-Protokoll – Änderungen
  immer an beiden vornehmen.
- Programme neu bauen (bettet die aktuelle Standardversion als Rückfall ein): `sh werkzeuge/server_bauen.sh`
  (benötigt Go). Lehrkräfte-Anleitung: `Server/Anleitung.html`.

## Veröffentlichung

- Öffentliches Repository: https://github.com/Majort0m0/TippTrainer (Branch `main`). Die PNS-Datei, `_backup_vor_fixes/`,
  Server-Daten (`TippTrainer-Daten/`) und Build-Ergebnisse sind per `.gitignore` ausgeschlossen – vor jedem Commit
  mit `git grep --cached` sicherstellen, dass keine Zugangsdaten enthalten sind.
- Fertige Programme nicht ins Repository, sondern als GitHub-Release: ZIPs mit Programm + `SchreibTrainer_4.0.html` +
  `Server/Anleitung.html` (ZIP erhält das Ausführungsrecht der macOS-Datei), dazu `SchreibTrainer_4.0.html` und `sync.php`.
