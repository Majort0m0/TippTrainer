#!/bin/sh
# Baut das TippTrainer-Server-Programm für Windows, macOS (Universal: Intel + Apple Silicon) und Linux
# (PC/x64, Raspberry Pi 64 Bit = arm64, Raspberry Pi 32 Bit = arm).
# Voraussetzung: Go (https://go.dev/dl/). Aufruf im Projektordner:  sh werkzeuge/server_bauen.sh
# Die aktuelle SchreibTrainer_4.0.html wird als Rückfall-Fassung ins Programm eingebettet.
set -e
cd "$(dirname "$0")/../Server/quellcode"
GO="${GO:-go}"
cp ../../SchreibTrainer_4.0.html app.html
FLAGS='-trimpath -ldflags=-s -ldflags=-w'

GOOS=windows GOARCH=amd64 $GO build $FLAGS -o ../TippTrainer-Server-Windows.exe .
GOOS=darwin GOARCH=arm64 $GO build $FLAGS -o /tmp/tts-arm64 .
GOOS=darwin GOARCH=amd64 $GO build $FLAGS -o /tmp/tts-amd64 .
lipo -create -output ../TippTrainer-Server-macOS /tmp/tts-arm64 /tmp/tts-amd64
rm -f /tmp/tts-arm64 /tmp/tts-amd64
codesign --force -s - ../TippTrainer-Server-macOS
chmod +x ../TippTrainer-Server-macOS

# Linux (reines Go, keine Abhängigkeiten): PC, Raspberry Pi 64 Bit (z. B. Pi 4/5), Raspberry Pi 32 Bit
GOOS=linux GOARCH=amd64 $GO build $FLAGS -o ../TippTrainer-Server-Linux-x64 .
GOOS=linux GOARCH=arm64 $GO build $FLAGS -o ../TippTrainer-Server-Linux-arm64 .
GOOS=linux GOARCH=arm GOARM=7 $GO build $FLAGS -o ../TippTrainer-Server-Linux-arm32 .
echo "Fertig: Server/TippTrainer-Server-Windows.exe, -macOS, -Linux-x64, -Linux-arm64, -Linux-arm32"
