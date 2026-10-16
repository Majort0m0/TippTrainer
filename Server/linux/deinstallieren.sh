#!/bin/sh
# Entfernt den TippTrainer-Server-Dienst.
# Die Daten (Ordner TippTrainer-Daten) bleiben erhalten, außer es wird  --mit-daten  angegeben.
set -e
if [ "$(id -u)" -ne 0 ]; then
    echo "Bitte mit Administratorrechten starten:  sudo sh deinstallieren.sh [--mit-daten]"
    exit 1
fi
systemctl stop tipptrainer 2>/dev/null || true
systemctl disable tipptrainer 2>/dev/null || true
rm -f /etc/systemd/system/tipptrainer.service
systemctl daemon-reload
if [ "$1" = "--mit-daten" ]; then
    rm -rf /opt/tipptrainer
    userdel tipptrainer 2>/dev/null || true
    echo "Dienst, Programm und Daten wurden entfernt."
else
    rm -f /opt/tipptrainer/tipptrainer-server /opt/tipptrainer/SchreibTrainer_4.0.html /opt/tipptrainer/Anleitung.html
    echo "Dienst und Programm wurden entfernt. Die Daten liegen weiter unter /opt/tipptrainer/TippTrainer-Daten."
    echo "Auch die Daten löschen:  sudo sh deinstallieren.sh --mit-daten"
fi
