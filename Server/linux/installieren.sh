#!/bin/sh
# Installiert den TippTrainer-Server als Dienst (systemd) auf Linux / Raspberry Pi.
# Aufruf im entpackten Ordner:   sudo sh installieren.sh
# Erneut ausgeführt aktualisiert das Skript das Programm und behält alle Daten.
set -e

if [ "$(id -u)" -ne 0 ]; then
    echo "Bitte mit Administratorrechten starten:  sudo sh installieren.sh"
    exit 1
fi
if ! command -v systemctl >/dev/null 2>&1; then
    echo "systemd wurde nicht gefunden. Das Programm lässt sich auch von Hand starten:  ./tipptrainer-server"
    exit 1
fi

cd "$(dirname "$0")"
ZIEL=/opt/tipptrainer
BENUTZER=tipptrainer

# Passt das Programm zu diesem Rechner (32/64 Bit, Intel/ARM)?
if ! ./tipptrainer-server --hilfe >/dev/null 2>&1; then
    echo "Dieses Paket passt nicht zu diesem Rechner (Architektur: $(uname -m))."
    echo "Raspberry Pi mit 64-Bit-System: Paket „RaspberryPi-64bit“, mit 32-Bit-System: „RaspberryPi-32bit“, normaler PC: „Linux-x64“."
    exit 1
fi

if ! id "$BENUTZER" >/dev/null 2>&1; then
    useradd --system --home-dir "$ZIEL" --shell /usr/sbin/nologin "$BENUTZER"
fi
mkdir -p "$ZIEL"

# Läuft schon eine ältere Fassung, kurz anhalten (Daten bleiben unangetastet)
systemctl stop tipptrainer 2>/dev/null || true

cp tipptrainer-server "$ZIEL/tipptrainer-server"
chmod 755 "$ZIEL/tipptrainer-server"
for datei in SchreibTrainer_4.0.html Anleitung.html; do
    [ -f "$datei" ] && cp "$datei" "$ZIEL/$datei"
done
chown -R "$BENUTZER":"$BENUTZER" "$ZIEL"
cp tipptrainer.service /etc/systemd/system/tipptrainer.service

# Einstellungen (Schlüssel, Admin-Passwort) einmal vor dem ersten Start anlegen
su -s /bin/sh "$BENUTZER" -c "cd '$ZIEL' && ./tipptrainer-server --passwort >/dev/null"

systemctl daemon-reload
systemctl enable tipptrainer >/dev/null 2>&1
systemctl restart tipptrainer
sleep 2

echo
if systemctl is-active --quiet tipptrainer; then
    echo "TippTrainer-Server läuft und startet künftig automatisch mit dem Rechner."
else
    echo "Der Dienst läuft nicht. Fehlersuche:  sudo journalctl -u tipptrainer -n 30"
    exit 1
fi
echo
su -s /bin/sh "$BENUTZER" -c "cd '$ZIEL' && ./tipptrainer-server --info"
echo
echo "Name im Netz (falls mDNS/Avahi aktiv ist):  http://$(hostname).local:8080"
echo "Später erneut anzeigen:   cd $ZIEL && sudo -u $BENUTZER ./tipptrainer-server --info"
echo "Admin-Passwort ändern:    cd $ZIEL && sudo -u $BENUTZER ./tipptrainer-server --passwort-setzen NEUESPASSWORT"
echo "Protokoll:                sudo journalctl -u tipptrainer -f"
