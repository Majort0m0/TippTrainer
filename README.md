# Tipp-Trainer (SchreibTrainer 4.0)

Lern-App zum Schreiben und Tippen für die Grundschule und Förderschule – eine einzelne HTML-Datei, die
vollständig im Browser läuft. Keine Installation, keine Benutzerkonten, kein Tracking.

**Online starten:** https://majort0m0.github.io/TippTrainer/ – oder [`SchreibTrainer_4.0.html`](SchreibTrainer_4.0.html) herunterladen und im Browser öffnen (funktioniert auch offline, nur ohne Übungsbilder).

## Funktionen

- **Übungsformen**
  - Wörterlisten: Bilddiktat, Hördiktat, Wortdiktat
  - Fließtexte: Lesen mit Vorlesefunktion und Bilder-Quiz, Lückentext, Abtippen (mit Wörtern pro Minute)
- **Korrektur:** Fehler werden genau markiert, es gibt einen zweiten Versuch, danach landet das Wort in den Lernwörtern. Die App erkennt Fehler bei Groß- und Kleinschreibung und eine eingeschaltete Feststelltaste.
- **Hilfen:** Vorlesen und Blitzlesen, Urkunde als PDF.
- **Papierkorb:** Gelöschte Gruppen, Profile und Lektionen bleiben 30 Tage im Papierkorb (Admin-Modus) und lassen sich wiederherstellen; er wird mit synchronisiert. Das Server-Programm hebt zusätzlich Tagessicherungen für 30 Tage auf.
- **Spiele-Belohnung (optional, standardmäßig aus):** Pro abgeschlossener Lektion gibt es eine Münze (höchstens 3). Eine Münze öffnet die Spielhalle mit vier Arcade-Spielen (Mindestspielzeit 1 Minute). Aktivierung pro Gruppe oder Profil im Admin-Modus.
- **Verwaltung (Admin-Modus):** Gruppen (auch mit Passwort und frei sortierbar), Profile, Lektions-Editor.
  - Bildersuche über ARASAAC (mit Grundformen/Wortstämmen) und Wikimedia, eigene Bilder hochladen
  - Umwandlung in Großbuchstaben
  - KI-Prompt zum Erstellen von Lektionen
  - Arbeitsblätter als PDF
- **Verteilen:** Lektionen an andere Profile übertragen, Vorlagen-Bibliothek, Import und Export als JSON, Gruppen- und Gesamt-Backup.
- **Export einzelner Übungen**
  - als eigenständiges **HTML-Widget**
  - als **SCORM-1.2-Paket** für Moodle und die ByCS Lernplattform
  - jeweils wahlweise als **Offline-Fassung** mit eingebetteten Bildern
- **Ohne CDN:** Alle Bausteine wie Gestaltung, Symbole, Schriften und PDF-Erzeugung sind in die Datei eingebettet. Internet wird nur für Übungsbilder gebraucht.
- **Synchronisation mehrerer Geräte** (optional), mit Drei-Wege-Abgleich, damit nichts überschrieben wird:
  - **Eigener Server:** Server-Programm für Windows, macOS, Linux und Raspberry Pi im Schulnetz (iPads verbinden sich per QR-Code oder Link) oder `sync.php` auf Webspace bzw. NAS
  - **Nextcloud** über WebDAV

## Eigener Server

Unter [Releases](../../releases) gibt es fertige Pakete:

- **TippTrainer-Server-Windows.zip** und **TippTrainer-Server-macOS.zip**: Server-Programm für einen PC im Schulnetz, mit App und Anleitung. Entpacken und doppelklicken; die iPads verbinden sich per QR-Code oder Link.
- **TippTrainer-Server-Linux-x64.tar.gz**, **TippTrainer-Server-RaspberryPi-64bit.tar.gz** und **-32bit.tar.gz**: dasselbe Programm für Linux und den Raspberry Pi, als Dienst mit Installationsskript (`sudo sh installieren.sh`), auch ohne Bildschirm bedienbar (`--info`, `--passwort`).
- **TippTrainer-Webspace-PHP.zip**: Ordner `tipptrainer` mit App, Startseite, `sync.php` und Anleitung für einen Webspace oder ein NAS mit PHP. Schlüssel in `sync.php` eintragen, den Ordner hochladen und `https://ihre-domain.de/tipptrainer/` öffnen.

**Verbindungsdaten im Browser:** Die Info-Seite des Servers (`/info`) zeigt Link und QR-Code für die Geräte und ist im Netz mit dem Admin-Passwort erreichbar – praktisch für einen Raspberry Pi ohne Bildschirm.

**Admin-Passwort:** Das Server-Programm setzt beim ersten Start das Standard-Passwort `lernsachen` für die App (änderbar), zeigt das Passwort auf der Info-Seite bzw. per Kommandozeile an und liefert der App nur einen Hash. Ohne Server-Programm (Einzel-Datei, Nextcloud, `sync.php`) legt die Lehrkraft beim ersten Admin-Zugriff ein eigenes Passwort fest (mit Notfall-Code); die PNS-Variante behält ihr festes Passwort.

Für den Raspberry Pi gibt es eine eigene Schritt-für-Schritt-Anleitung: [`Server/Raspberry-Pi-Anleitung.html`](Server/Raspberry-Pi-Anleitung.html) (Installation, Inbetriebnahme, Alltag, Netzwechsel, Fehlersuche).

Die Anleitung für Lehrkräfte steht in [`Server/Anleitung.html`](Server/Anleitung.html).

## Entwicklung

Bearbeitet wird nur `SchreibTrainer_4.0.html`. Danach führt man diese Werkzeuge aus:

| Werkzeug | Zweck |
|---|---|
| `python3 werkzeuge/einbetten.py` | bettet Tailwind (vorkompiliert), Font Awesome, Schriften, html2pdf.js und JSZip neu ein (benötigt Node/npx) |
| `python3 werkzeuge/pns_erzeugen.py` | erzeugt eine lokale Variante mit fest hinterlegten Nextcloud-Zugangsdaten (nicht im Repository) |
| `sh werkzeuge/server_bauen.sh` | baut das Server-Programm für Windows, macOS und Linux/Raspberry Pi (benötigt Go) |
| `sh werkzeuge/release_pakete.sh` | erstellt die Download-Pakete für ein Release im Ordner `dist/` |

Weitere Hinweise zur Code-Pflege stehen in [`CLAUDE.md`](CLAUDE.md).

## Lizenz und Drittanbieter

Tipp-Trainer: CC BY-NC – Lernsachen.blog (siehe Hinweis in der App).

Eingebettet bzw. verwendet werden:
- [Tailwind CSS](https://tailwindcss.com) (MIT)
- [Font Awesome Free](https://fontawesome.com) (Icons CC BY 4.0, Schriften SIL OFL 1.1, Code MIT)
- die Schriften [Comic Neue](https://comicneue.com) und [Andika](https://software.sil.org/andika/) (SIL OFL 1.1)
- [html2pdf.js](https://github.com/eKoopmans/html2pdf.js) (MIT)
- [JSZip](https://stuk.github.io/jszip/) (MIT/GPLv3)
- [rsc.io/qr](https://pkg.go.dev/rsc.io/qr) (BSD)

Übungsbilder: [ARASAAC](https://arasaac.org) (CC BY-NC-SA), [Wikimedia](https://commons.wikimedia.org).
