<?php
/*
 * TippTrainer – Sync-Endpunkt für eigenen Webspace, NAS oder Schul-PC (PHP ≥ 7.4)
 *
 * Einrichtung:
 *  1. Unten bei $SCHLUESSEL einen eigenen, langen Schlüssel eintragen (mind. 12 Zeichen).
 *  2. Diese Datei auf den Webspace laden – am besten in denselben Ordner wie die
 *     SchreibTrainer-HTML-Datei. Der Ordner muss für PHP beschreibbar sein.
 *  3. In der App: Speicher-Symbol → „Eigener Server“ → Adresse dieser Datei + Schlüssel.
 *
 * Die Daten landen im Unterordner „tipptrainer-daten“ (wird automatisch angelegt und per
 * .htaccess gesperrt). Bei nginx-Servern den Ordner zusätzlich in der Server-Konfiguration
 * sperren oder $DATENORDNER auf einen Pfad außerhalb des Web-Verzeichnisses setzen.
 *
 * Protokoll (identisch zum TippTrainer-Server-Programm):
 *   GET  → 200 + JSON + X-TippTrainer-Version | 404 (noch keine Daten)
 *   PUT  mit X-TippTrainer-If-Match: <version> | "*none*" → 200 | 412 (anderes Gerät war schneller)
 *   Schlüssel im Header X-TippTrainer-Key, sonst 401
 */

$SCHLUESSEL  = 'HIER-EIGENEN-SCHLUESSEL-EINTRAGEN';
$DATENORDNER = __DIR__ . '/tipptrainer-daten';
$SICHERUNGEN = 50;          // so viele ältere Fassungen werden aufbewahrt
$MAX_GROESSE = 50 * 1024 * 1024;

// ------------------------------------------------------------------------------------

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Methods: GET, PUT, OPTIONS');
header('Access-Control-Allow-Headers: Content-Type, X-TippTrainer-Key, X-TippTrainer-If-Match');
header('Access-Control-Expose-Headers: X-TippTrainer-Version');
header('Access-Control-Allow-Private-Network: true');
header('Cache-Control: no-store');

function antwort($code, $nachricht) {
    http_response_code($code);
    header('Content-Type: application/json; charset=utf-8');
    echo json_encode(['tipptrainerSync' => 1, 'fehler' => $nachricht], JSON_UNESCAPED_UNICODE);
    exit;
}

function version_von($inhalt) {
    return substr(hash('sha256', $inhalt), 0, 24);
}

$methode = $_SERVER['REQUEST_METHOD'] ?? 'GET';
if ($methode === 'OPTIONS') { http_response_code(204); exit; }

if (strlen($SCHLUESSEL) < 12 || $SCHLUESSEL === 'HIER-EIGENEN-SCHLUESSEL-EINTRAGEN') {
    antwort(500, 'sync.php ist noch nicht eingerichtet: bitte einen eigenen Schlüssel eintragen');
}
$schluessel = $_SERVER['HTTP_X_TIPPTRAINER_KEY'] ?? '';
if (!hash_equals($SCHLUESSEL, $schluessel)) {
    antwort(401, 'Schlüssel fehlt oder ist falsch');
}

if (!is_dir($DATENORDNER . '/Sicherungen') && !@mkdir($DATENORDNER . '/Sicherungen', 0700, true)) {
    antwort(500, 'Datenordner kann nicht angelegt werden (Schreibrechte prüfen)');
}
if (!file_exists($DATENORDNER . '/.htaccess')) {
    @file_put_contents($DATENORDNER . '/.htaccess', "Require all denied\nDeny from all\n");
}
if (!file_exists($DATENORDNER . '/index.html')) {
    @file_put_contents($DATENORDNER . '/index.html', '');
}

$datei = $DATENORDNER . '/daten.json';
$sperre = fopen($DATENORDNER . '/.sperre', 'c');
if (!$sperre) antwort(500, 'Sperrdatei nicht verfügbar');

if ($methode === 'GET') {
    flock($sperre, LOCK_SH);
    $inhalt = @file_get_contents($datei);
    flock($sperre, LOCK_UN);
    if ($inhalt === false) antwort(404, 'Noch keine Daten vorhanden');
    header('Content-Type: application/json; charset=utf-8');
    header('X-TippTrainer-Version: ' . version_von($inhalt));
    echo $inhalt;
    exit;
}

if ($methode === 'PUT') {
    $neu = file_get_contents('php://input', false, null, 0, $MAX_GROESSE + 1);
    if ($neu === false || strlen($neu) > $MAX_GROESSE) antwort(413, 'Daten zu groß');
    $test = json_decode($neu, true);
    if (!is_array($test)) antwort(400, 'Ungültige Daten');

    $bedingung = $_SERVER['HTTP_X_TIPPTRAINER_IF_MATCH'] ?? '';
    flock($sperre, LOCK_EX);
    $alt = @file_get_contents($datei);
    $existiert = $alt !== false;

    if ($bedingung === '*none*' && $existiert) { flock($sperre, LOCK_UN); antwort(412, 'Daten wurden inzwischen angelegt'); }
    if ($bedingung !== '' && $bedingung !== '*none*' && (!$existiert || !hash_equals(version_von($alt), $bedingung))) {
        flock($sperre, LOCK_UN); antwort(412, 'Daten wurden inzwischen geändert');
    }

    // Ältere Fassung sichern (höchstens eine pro Minute), alte Sicherungen aufräumen
    if ($existiert && $alt !== $neu) {
        @file_put_contents($DATENORDNER . '/Sicherungen/daten-' . date('Y-m-d_H-i') . '.json', $alt);
        $liste = glob($DATENORDNER . '/Sicherungen/daten-*.json') ?: [];
        sort($liste);
        while (count($liste) > $SICHERUNGEN) @unlink(array_shift($liste));
    }

    $tmp = $datei . '.tmp';
    if (@file_put_contents($tmp, $neu) === false || !@rename($tmp, $datei)) {
        flock($sperre, LOCK_UN); antwort(500, 'Speichern fehlgeschlagen (Schreibrechte prüfen)');
    }
    flock($sperre, LOCK_UN);
    header('Content-Type: application/json; charset=utf-8');
    header('X-TippTrainer-Version: ' . version_von($neu));
    echo '{"ok":true}';
    exit;
}

antwort(405, 'Methode nicht erlaubt');
