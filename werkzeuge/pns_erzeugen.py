#!/usr/bin/env python3
"""Erzeugt die PNS-Variante aus der Standardversion.

Die PNS-Datei ist inhaltlich identisch mit SchreibTrainer_4.0.html – bis auf zwei Zeilen:
  - const EMBEDDED_WEBDAV_CONFIG = { ...fest hinterlegte Nextcloud-Zugangsdaten... };
  - const ADMIN_PASSWORD = "...";

Dieses Skript liest beide Werte aus der BESTEHENDEN PNS-Datei (die Zugangsdaten stehen also
nirgends sonst) und schreibt die PNS-Datei neu: Standardversion + diese beiden Werte.

Aufruf (im Projektordner):  python3 werkzeuge/pns_erzeugen.py
"""
import os
import re
import sys

ORDNER = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
STANDARD = os.path.join(ORDNER, 'SchreibTrainer_4.0.html')
PNS = os.path.join(ORDNER, 'SchreibTrainer_4.0_Nextcloud-PNS.html')

RE_CONFIG = re.compile(r'^[ \t]*const EMBEDDED_WEBDAV_CONFIG = (?:null|\{[\s\S]*?\});', re.M)
RE_ADMIN = re.compile(r'^[ \t]*const ADMIN_PASSWORD = "[^"\n]*";', re.M)


def einmal(regex, text, wo):
    treffer = regex.findall(text)
    if len(treffer) != 1:
        sys.exit(f'Abbruch: {regex.pattern!r} in {wo} {len(treffer)}x gefunden (erwartet: 1).')
    return treffer[0]


def main():
    standard = open(STANDARD, encoding='utf-8').read()
    pns_alt = open(PNS, encoding='utf-8').read()

    config = einmal(RE_CONFIG, pns_alt, 'PNS-Datei')
    admin = einmal(RE_ADMIN, pns_alt, 'PNS-Datei')
    if config.rstrip().endswith('= null;'):
        sys.exit('Abbruch: Die PNS-Datei enthält keine Zugangsdaten (EMBEDDED_WEBDAV_CONFIG = null).')
    einmal(RE_CONFIG, standard, 'Standardversion')
    einmal(RE_ADMIN, standard, 'Standardversion')

    neu = RE_CONFIG.sub(lambda m: config, standard, count=1)
    neu = RE_ADMIN.sub(lambda m: admin, neu, count=1)

    with open(PNS, 'w', encoding='utf-8') as f:
        f.write(neu)
    os.chmod(PNS, 0o600)
    print('PNS-Variante aktualisiert:', os.path.basename(PNS))


if __name__ == '__main__':
    main()
