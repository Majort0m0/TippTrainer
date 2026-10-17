#!/bin/sh
# Erstellt die Download-Pakete für ein GitHub-Release im Ordner dist/:
#   TippTrainer-Server-Windows.zip, TippTrainer-Server-macOS.zip  (Programm + App + Anleitung)
#   TippTrainer-Server-Linux-x64.tar.gz, TippTrainer-Server-RaspberryPi-64bit.tar.gz, -32bit.tar.gz
#                                              (Programm + App + Anleitung + Dienst-Datei + Installationsskript)
#   TippTrainer-Webspace-PHP.zip                                   (Ordner tipptrainer: App, index.html, sync.php, Anleitung)
#   SchreibTrainer_4.0.html, sync.php                               (einzeln)
# Vorher die Programme bauen: sh werkzeuge/server_bauen.sh
# Hochladen z.B.: gh release upload <tag> dist/* --clobber
set -e
cd "$(dirname "$0")/.."
ROOT="$(pwd)"
rm -rf dist && mkdir -p dist/tmp

paket_server() { # $1 = Name im ZIP, $2 = Programmdatei
    rm -rf dist/tmp/* && mkdir -p dist/tmp/TippTrainer-Server
    cp "Server/$2" SchreibTrainer_4.0.html Server/Anleitung.html dist/tmp/TippTrainer-Server/
    (cd dist/tmp && zip -qryX "$ROOT/dist/$1" TippTrainer-Server)
}
paket_server TippTrainer-Server-Windows.zip TippTrainer-Server-Windows.exe
paket_server TippTrainer-Server-macOS.zip TippTrainer-Server-macOS

paket_linux() { # $1 = Name des Pakets, $2 = Programmdatei (Server/…)
    rm -rf dist/tmp/* && mkdir -p dist/tmp/TippTrainer-Server
    cp "Server/$2" dist/tmp/TippTrainer-Server/tipptrainer-server
    cp SchreibTrainer_4.0.html Server/Anleitung.html Server/Raspberry-Pi-Anleitung.html Server/linux/tipptrainer.service Server/linux/installieren.sh Server/linux/deinstallieren.sh dist/tmp/TippTrainer-Server/
    chmod 755 dist/tmp/TippTrainer-Server/tipptrainer-server
    # ohne macOS-Metadaten (Quarantäne-Flags u. Ä.), sonst meldet tar unter Linux Warnungen
    (cd dist/tmp && COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata -czf "$ROOT/dist/$1" TippTrainer-Server 2>/dev/null || COPYFILE_DISABLE=1 tar -czf "$ROOT/dist/$1" TippTrainer-Server)
}
paket_linux TippTrainer-Server-Linux-x64.tar.gz TippTrainer-Server-Linux-x64
paket_linux TippTrainer-Server-RaspberryPi-64bit.tar.gz TippTrainer-Server-Linux-arm64
paket_linux TippTrainer-Server-RaspberryPi-32bit.tar.gz TippTrainer-Server-Linux-arm32

rm -rf dist/tmp/* && mkdir -p dist/tmp/tipptrainer
cp SchreibTrainer_4.0.html index.html Server/sync.php Server/Anleitung.html dist/tmp/tipptrainer/
(cd dist/tmp && zip -qr "$ROOT/dist/TippTrainer-Webspace-PHP.zip" tipptrainer)

rm -rf dist/tmp
cp SchreibTrainer_4.0.html Server/sync.php Server/Raspberry-Pi-Anleitung.html dist/
ls -l dist
