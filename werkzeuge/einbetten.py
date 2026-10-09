#!/usr/bin/env python3
"""Bettet alle Fremd-Bausteine direkt in SchreibTrainer_4.0.html ein (keine CDN-/Google-Aufrufe mehr):

  - Tailwind CSS   → aus den tatsächlich verwendeten Klassen vorkompiliert (benötigt Node/npx)
  - Font Awesome   → CSS + Schriften (Solid, Regular) als base64
  - Schriften      → Comic Neue (400, 700) und Andika (400), latin + latin-ext, als base64
  - html2pdf.js    → PDF-Erzeugung (Urkunde, Arbeitsblatt)
  - JSZip          → SCORM-Export (wird aus Widget-Exporten wieder entfernt)

Die Blöcke stehen zwischen <!-- EINGEBETTET:name START … --> und <!-- EINGEBETTET:name ENDE -->
und werden bei jedem Aufruf neu erzeugt. Nach Änderungen an der Oberfläche (neue Tailwind-Klassen)
erneut ausführen, danach werkzeuge/pns_erzeugen.py.

Aufruf im Projektordner:  python3 werkzeuge/einbetten.py
"""
import base64
import io
import json
import os
import re
import subprocess
import sys
import tarfile
import tempfile
import urllib.request

ORDNER = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DATEI = os.environ.get('TT_DATEI') or os.path.join(ORDNER, 'SchreibTrainer_4.0.html')
CACHE = os.path.join(ORDNER, 'werkzeuge', '.cache')

PAKETE = {
    'fontawesome': ('@fortawesome/fontawesome-free', '6.4.0'),
    'comicneue': ('@fontsource/comic-neue', '5.2.5'),
    'andika': ('@fontsource/andika', '5.2.5'),
    'html2pdf': ('html2pdf.js', '0.10.1'),
    'jszip': ('jszip', '3.10.1'),
}
TAILWIND = 'tailwindcss@3.4.17'

HINWEIS = 'automatisch erzeugt von werkzeuge/einbetten.py – nicht von Hand bearbeiten'


def paket(name, version):
    """Lädt ein npm-Paket (einmalig) und gibt den Pfad zum entpackten Ordner zurück."""
    ziel = os.path.join(CACHE, name.replace('/', '__') + '-' + version)
    if os.path.isdir(ziel):
        return ziel
    os.makedirs(ziel, exist_ok=True)
    kurz = name.split('/')[-1]
    url = f'https://registry.npmjs.org/{name}/-/{kurz}-{version}.tgz'
    print('  lade', url)
    daten = urllib.request.urlopen(url).read()
    with tarfile.open(fileobj=io.BytesIO(daten), mode='r:gz') as tar:
        for m in tar.getmembers():
            if not m.isfile():
                continue
            rel = m.name.split('/', 1)[1] if '/' in m.name else m.name
            if rel.startswith('/') or '..' in rel.split('/'):
                continue
            pfad = os.path.join(ziel, rel)
            os.makedirs(os.path.dirname(pfad), exist_ok=True)
            with open(pfad, 'wb') as f:
                f.write(tar.extractfile(m).read())
    return ziel


def b64(pfad, mime):
    with open(pfad, 'rb') as f:
        return f'data:{mime};base64,' + base64.b64encode(f.read()).decode()


def lies(pfad):
    with open(pfad, encoding='utf-8') as f:
        return f.read()


def script_sicher(js):
    # Inline-Script darf weder </script noch <!-- enthalten (HTML-Parser)
    js = re.sub(r'</(script)', r'<\\/\1', js, flags=re.I)
    return js.replace('<!--', '<\\!--')


def schriften_css():
    teile = []
    for key, familie, gewichte in (('comicneue', 'comic-neue', (400, 700)), ('andika', 'andika', (400,))):
        ordner = paket(*PAKETE[key])
        for g in gewichte:
            css = lies(os.path.join(ordner, f'{g}.css'))
            for block in re.findall(r'@font-face\s*\{[^}]*\}', css):
                datei = re.search(r'url\(\./files/([^)]+\.woff2)\)', block)
                if not datei:
                    continue
                subset = datei.group(1)
                if not (f'-latin-{g}-' in subset or f'-latin-ext-{g}-' in subset):
                    continue
                daten = b64(os.path.join(ordner, 'files', datei.group(1)), 'font/woff2')
                neu = re.sub(r'src:[^;]+;', f'src:url({daten}) format("woff2");', block)
                teile.append(neu)
    return '<style>' + '\n'.join(teile) + '</style>'


def fontawesome_css():
    ordner = paket(*PAKETE['fontawesome'])
    css = ''
    for n in ('fontawesome.min.css', 'solid.min.css', 'regular.min.css'):
        css += lies(os.path.join(ordner, 'css', n))

    def ersetze(m):
        datei = m.group(1)
        return f'src:url({b64(os.path.join(ordner, "webfonts", datei), "font/woff2")}) format("woff2")'
    css = re.sub(r'src:url\(\.\./webfonts/([\w-]+\.woff2)\) format\("woff2"\),url\([^)]+\) format\("truetype"\)', ersetze, css)
    if '../webfonts/' in css:
        sys.exit('Abbruch: Font-Awesome-CSS enthält noch externe Schriftverweise.')
    return '<style>' + css + '</style>'


def script_block(key, pfad_im_paket):
    ordner = paket(*PAKETE[key])
    return '<script>' + script_sicher(lies(os.path.join(ordner, pfad_im_paket))) + '</script>'


def ohne_bloecke(html):
    return re.sub(r'<!-- EINGEBETTET:(\w+) START[\s\S]*?<!-- EINGEBETTET:\1 ENDE -->', '', html)


def tailwind_css(html):
    with tempfile.TemporaryDirectory() as tmp:
        quelle = os.path.join(tmp, 'inhalt.html')
        with open(quelle, 'w', encoding='utf-8') as f:
            f.write(ohne_bloecke(html))
        eingabe = os.path.join(tmp, 'in.css')
        with open(eingabe, 'w') as f:
            f.write('@tailwind base;\n@tailwind components;\n@tailwind utilities;\n')
        ausgabe = os.path.join(tmp, 'out.css')
        subprocess.run(['npx', '-y', TAILWIND, '-i', eingabe, '-o', ausgabe, '--content', quelle, '--minify'],
                       check=True, cwd=tmp, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        return '<style>' + lies(ausgabe) + '</style>'


def block(name, inhalt):
    return f'<!-- EINGEBETTET:{name} START ({HINWEIS}) -->\n    {inhalt}\n    <!-- EINGEBETTET:{name} ENDE -->'


def setze(html, name, inhalt):
    muster = re.compile(r'<!-- EINGEBETTET:' + name + r' START[\s\S]*?<!-- EINGEBETTET:' + name + r' ENDE -->')
    if len(muster.findall(html)) != 1:
        sys.exit(f'Abbruch: Block {name} nicht genau einmal gefunden.')
    return muster.sub(lambda m: block(name, inhalt), html)


def erstmalig_umstellen(html):
    """Ersetzt die CDN-Einbindungen einmalig durch (leere) Block-Markierungen."""
    if '<!-- EINGEBETTET:tailwind START' in html:
        return html
    ersetzungen = [
        ('    <script src="https://cdn.tailwindcss.com"></script>\n', ''),
        ('    <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css" rel="stylesheet">\n', ''),
        ('    <script src="https://cdnjs.cloudflare.com/ajax/libs/html2pdf.js/0.10.1/html2pdf.bundle.min.js"></script>\n', ''),
        ("        @import url('https://fonts.googleapis.com/css2?family=Comic+Neue:wght@400;700&display=swap');\n", ''),
        ("        @import url('https://fonts.googleapis.com/css2?family=Andika&display=swap');\n", ''),
    ]
    for alt, neu in ersetzungen:
        if html.count(alt) != 1:
            sys.exit('Abbruch: erwartete CDN-Einbindung nicht gefunden: ' + alt.strip())
        html = html.replace(alt, neu)
    kopf = ''.join('    ' + block(n, '') + '\n' for n in ('schriften', 'fontawesome', 'html2pdf', 'jszip'))
    html = html.replace('    <style>\n', kopf + '    <style>\n', 1)
    html = html.replace('</head>', '    ' + block('tailwind', '') + '\n</head>', 1)
    return html


def main():
    html = erstmalig_umstellen(lies(DATEI))
    print('Schriften …');     html = setze(html, 'schriften', schriften_css())
    print('Font Awesome …');  html = setze(html, 'fontawesome', fontawesome_css())
    print('html2pdf …');      html = setze(html, 'html2pdf', script_block('html2pdf', 'dist/html2pdf.bundle.min.js'))
    print('JSZip …');         html = setze(html, 'jszip', script_block('jszip', 'dist/jszip.min.js'))
    print('Tailwind …');      html = setze(html, 'tailwind', tailwind_css(html))

    rest = re.findall(r'(?:src|href)="https://(?:cdn|fonts)\.[^"]+"|@import url\(', ohne_bloecke(html))
    if rest:
        sys.exit('Abbruch: noch externe Einbindungen gefunden: ' + ', '.join(rest))
    with open(DATEI, 'w', encoding='utf-8') as f:
        f.write(html)
    print(f'Fertig: {os.path.basename(DATEI)} ({len(html.encode()) // 1024} KB)')


if __name__ == '__main__':
    main()
