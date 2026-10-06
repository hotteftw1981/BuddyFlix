# 🎬 BuddyFlix

> ⚠️ **Development Branch**
>
> Dieser Branch enthält unfertige, experimentelle und noch nicht vollständig getestete Funktionen.
> Für den stabilen Stand bitte **`main`** verwenden.

> **„Wenn Jellyfin auf ARMHF stehen bleibt, bauen wir halt selbst weiter.“**

BuddyFlix ist ein schlanker, moderner Media Server mit Fokus auf ältere und schwächere NAS-Hardware – begonnen auf einem **QNAP TS-431P2 (ARMHF / ARMv7, 8 GB RAM)**.

Das Projekt entstand aus einer einfachen Idee:

> **„Ich will mehr. Ich will weiter. Also machen wir was Eigenes.“**

Und weil wir offensichtlich nicht genug Projekte gleichzeitig haben:

> **„Mit schlafen ist nicht so viel.“** 😅

---

## 🚧 Status

**Stabiler Stand auf `main`: V0.1.0**

**Aktueller Entwicklungsstand: V0.1.1-dev**

V0.1.0 ist weiterhin nur der bestätigte QNAP-Proof-of-Concept. Alles, was unten unter **„In develop implementiert“** steht, ist bereits im Entwicklungsbranch vorhanden, gilt aber ausdrücklich **noch nicht als stabil veröffentlicht**, solange es nicht praktisch getestet und anschließend nach `main` übernommen wurde.

### ✅ In V0.1.0 tatsächlich bestätigt

- Installation als QNAP-QPKG über das App Center
- Start auf QNAP TS-431P2 / ARMHF / ARMv7
- BuddyFlix-Weboberfläche erreichbar
- Login funktioniert
- Backend läuft auf Port 8096
- statisches ARMv7-Binary
- erste Dark-Mode-Oberfläche
- grundlegende Server-API vorhanden

### 🧪 In develop implementiert, aber noch nicht als stabil freigegeben

- First-Run-Setup mit eigenem Servernamen, Admin-Benutzer und Passwort
- persistente Servereinstellungen
- Admin-Passwort ändern
- konfigurierbarer Servername
- Bibliotheken hinzufügen, bearbeiten und entfernen
- Bibliothekstypen für Filme, Serien und andere Videos
- Bibliotheksscans
- robustere Scans bei nicht erreichbaren Bibliothekspfaden
- Erkennung und getrennte Behandlung fehlender/verwaister Medien
- QNAP-/NAS-Systemordner werden beim Scan übersprungen
- Scan-Diagnose mit Anzahl gefundener Mediendateien und übersprungener Systemordner
- erweiterte Verwaltungsoberfläche
- Unterstützung für ein zentrales BuddyFlix-TheTVDB-Projektcredential im Build
- optionaler eigener TheTVDB-Key nur noch als Server-Override
- Medien manuell bearbeiten
- Medienstatus gesehen / ungesehen
- Wiedergabefortschritt zurücksetzen
- Bereinigung verwaister Medieneinträge
- TheTVDB-Suche mit auswählbaren Treffern
- Übernahme eines gezielt ausgewählten TheTVDB-Treffers
- automatische TheTVDB-Metadatenerkennung für komplette Bibliotheken
- konservative Auto-Zuordnung eindeutiger Treffer; unsichere Treffer landen in „Bitte prüfen“
- provider-neutrale BuddyFlix Metadata Engine als Basis für weitere Metadatenquellen
- persistente externe IDs (TheTVDB und – sofern vorhanden – IMDb/TMDb)
- Herkunftsangaben für übernommene Metadatenfelder
- Client-/Discovery-Endpunkte für spätere Apps
- dynamische Versionsanzeige im Frontend
- eindeutige Dev-Buildnummern für QNAP-Testpakete
- GitHub CI mit Tests, JavaScript-Syntaxcheck sowie AMD64- und ARMHF-Build
- automatische Erzeugung eines installierbaren QNAP-ARMHF-Dev-QPKG als GitHub-Actions-Artifact
- neues BuddyFlix-Logo in Weboberfläche und Anmeldeseiten
- eigenes BuddyFlix-App-Icon für QNAP-QPKG

**Wichtig:** Diese Liste beschreibt den aktuellen Entwicklungsstand des Codes – nicht automatisch den Stand eines veröffentlichten QPKG.

> **„Erst testen, dann angeben.“** 😄

---

## 🔑 Anmeldung

### Stabiler Stand V0.1.0

```text
Benutzer: admin
Passwort: buddyflix
```

> **Bitte das Standardpasswort nicht für einen öffentlich erreichbaren Server verwenden.**

### develop / V0.1.1-dev

Bei einer frischen Installation ist eine **Ersteinrichtung** vorgesehen. Dabei werden Servername, Admin-Benutzer und Passwort selbst vergeben. Ein eigener Provider-Key gehört nicht zur normalen Ersteinrichtung. BuddyFlix kann stattdessen ein zentrales TheTVDB-Projektcredential im Build verwenden; ein eigener TheTVDB-Key bleibt nur als optionaler Override möglich.

---

## 🎯 Ziel

BuddyFlix soll **kein 1:1-Jellyfin-Klon** werden.

Das Ziel ist ein eigener, ressourcenschonender Media Server mit bewusstem Fokus auf:

- ältere QNAP-/Synology-/ARM-NAS-Systeme
- Direct Play statt unnötigem Transcoding
- geringe CPU-Last
- geringe RAM-Last
- saubere Weboberfläche
- einfache Installation
- moderne UX
- Multiarch-Builds

### 🔥 Hohe Priorität

- **Native Fire TV Stick App**
  - TV-optimierte Oberfläche
  - Steuerung vollständig per Fire-TV-Fernbedienung
  - Server automatisch im Heimnetz finden
  - Benutzer-/Profilwahl
  - Startseite, Filme, Serien und „Weiterschauen“
  - Direct Play so oft wie möglich
  - Untertitel- und Tonspurwahl
  - Wiedergabefortschritt mit dem BuddyFlix-Server synchronisieren
  - später möglichst bequem als APK sideloadbar und perspektivisch Amazon Appstore

> **„Was bringt der schönste Media Server, wenn wir ihn nicht gemütlich vom Sofa aus benutzen können?“** 😄

Perspektivisch geplant:

- Serien
- Staffeln & Episoden
- Favoriten
- Benutzerprofile
- Sammlungen
- Untertitel
- Direct Stream / Remux
- Audio-Transcoding
- Video-Transcoding als letzte Option
- **Fire TV Stick App**
- Android-/Android-TV-Client
- PWA
- erweiterte Systemüberwachung
- NAS Protection / Lastbegrenzung
- BuddyFlix-Splashscreen/Startbild für Web und Fire TV

---

## 🧠 Designprinzip

BuddyFlix versucht zuerst immer, so wenig wie möglich zu rechnen:

```
Direct Play
    ↓
Direct Stream / Remux
    ↓
Audio Transcoding
    ↓
Video Transcoding
```

Oder anders gesagt:

> **„Wir versuchen nicht, auf vier Cortex-A15-Kernen Netflix' Rechenzentrum nachzuspielen.“**

---

## 🖥️ Referenzhardware

Entwicklung und erster bestätigter QNAP-Test:

- **QNAP TS-431P2**
- AnnapurnaLabs Alpine AL-314
- 4 × 1,7 GHz Cortex-A15
- ARMv7 / ARMHF
- 8 GB RAM

BuddyFlix wird so entwickelt, dass dieses Gerät als Referenz für schwächere ARMHF-Hardware dient.

---

## 📦 QNAP

BuddyFlix wird als **.qpkg** für QNAP vorbereitet.

Installation des stabilen Pakets:

1. QTS öffnen
2. App Center
3. „Manuell installieren“
4. BuddyFlix-QPKG auswählen
5. installieren
6. BuddyFlix im Browser auf Port 8096 öffnen

Die V0.1.0-QPKG-Struktur wurde auf einem TS-431P2 erfolgreich installiert und gestartet.

Für `develop` werden fortlaufend eindeutig versionierte Testpakete gebaut, z. B. `0.1.1-dev.36`. Diese Builds sind **keine Stable Releases** und dienen ausschließlich zum Testen des aktuellen Entwicklungsstands.

---

## 🔐 Hinweis

BuddyFlix befindet sich aktuell in einer frühen Entwicklungsphase und ist noch **nicht für den offenen Internetbetrieb gedacht**.

Bitte derzeit bevorzugt nur im internen Netzwerk verwenden.

---

### Metadatenquelle

BuddyFlix verwendet im Entwicklungszweig **TheTVDB** als primären Metadatenprovider.

Metadata provided by [TheTVDB](https://thetvdb.com). Please consider adding missing information or subscribing.

TMDb bleibt vorerst nur als technische Fallback-Implementierung im Code und ist nicht der primäre BuddyFlix-Provider.

---

## 🛠️ Technik

Aktuell:

- Go Backend
- statisches ARMv7-Binary
- Weboberfläche
- lokale Persistenz
- QNAP-QPKG-Paketierung
- Direct-Play-orientierte Architektur
- GitHub Actions für Tests und Dev-Paketbau

In Entwicklung:

- Serienstruktur
- SQLite
- ffprobe / FFmpeg Integration
- Multiarch-Builds
- automatisierte Stable Releases
- Fire-TV-Client

---

## 🧪 Philosophie

BuddyFlix entsteht nicht als akademische Übung, sondern als echte laufende Anwendung auf echter alter Hardware.

Das bedeutet:

- erst testen
- dann schönreden 😄
- keine unnötigen Abhängigkeiten
- keine riesigen Frameworks, wenn es auch klein geht
- lieber schlau arbeiten als rohe CPU-Leistung voraussetzen
- stabile README = nur bestätigte Funktionen
- develop-README = transparenter Entwicklungsstand mit klarer Kennzeichnung

Oder inoffiziell:

> **„Wenn das TS-431P2 nicht stirbt, haben wir alles richtig gemacht.“**

---

## 📌 Changelog

### V0.1.1-dev

Aktueller Entwicklungszweig. Noch nicht als stabil veröffentlicht.

- First-Run-Setup
- persistente Servereinstellungen
- ausgebaute Administration
- Bibliotheksverwaltung
- robustere QNAP-Bibliotheksscans
- fehlende/verwaiste Medien getrennt vom normalen Filmbestand
- NAS-Systemordner beim Scan ausgeschlossen
- dynamische Dev-Buildnummern
- installierbare QNAP-ARMHF-Dev-QPKGs über GitHub Actions
- neues BuddyFlix-Branding mit Web-Logo und QNAP-App-Icon
- TheTVDB-Konfiguration
- Medieneditor
- Medienstatus und Bereinigung
- auswählbare TheTVDB-Treffer
- automatische TheTVDB-Metadatenläufe mit Prüfliste für unsichere Treffer
- provider-neutrale Metadata Engine mit externen IDs und Quellenherkunft
- Build-Unterstützung für ein zentrales BuddyFlix-TheTVDB-Projektcredential; Nutzer-Key nur noch optionaler Override
- erste Client-/Discovery-API
- CI für Go-Tests, JavaScript-Syntaxcheck, AMD64 und ARMHF

### V0.1.0

Erster bestätigter QNAP-Proof-of-Concept.

- ARMHF-/ARMv7-Serverbasis
- Login
- Dark-Mode-Weboberfläche
- BuddyFlix startet auf einem QNAP TS-431P2
- Installation über QNAP App Center erfolgreich getestet
- Port 8096 erreichbar
- Basis für Scanner, Streaming und Medienverwaltung im Code vorbereitet

**Wichtig:** V0.1.0 ist noch kein fertiger Media Server, sondern der erste lauffähige Grundstein.

---

## ❤️ Projektstatus

**Experimental / Early Development**

Das Projekt ist öffentlich, weil wir finden:

> **„Wenn wir schon so einen Unsinn bauen, dann wenigstens öffentlich.“** 😄

Beiträge, Tests, Ideen und Bugreports sind willkommen.
