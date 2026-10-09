#!/bin/sh
# Erstellt die Download-Pakete für ein GitHub-Release im Ordner dist/:
#   TippTrainer-Server-Windows.zip, TippTrainer-Server-macOS.zip  (Programm + App + Anleitung)
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
    (cd dist/tmp && zip -qry "$ROOT/dist/$1" TippTrainer-Server)
}
paket_server TippTrainer-Server-Windows.zip TippTrainer-Server-Windows.exe
paket_server TippTrainer-Server-macOS.zip TippTrainer-Server-macOS

rm -rf dist/tmp/* && mkdir -p dist/tmp/tipptrainer
cp SchreibTrainer_4.0.html index.html Server/sync.php Server/Anleitung.html dist/tmp/tipptrainer/
(cd dist/tmp && zip -qr "$ROOT/dist/TippTrainer-Webspace-PHP.zip" tipptrainer)

rm -rf dist/tmp
cp SchreibTrainer_4.0.html Server/sync.php dist/
ls -l dist
