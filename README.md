# 🎬 BuddyFlix

> **„Wenn Jellyfin auf ARMHF stehen bleibt, bauen wir halt selbst weiter.“**

BuddyFlix ist ein schlanker, moderner Media Server mit Fokus auf ältere und schwächere NAS-Hardware – begonnen auf einem **QNAP TS-431P2 (ARMHF / ARMv7, 8 GB RAM)**.

Das Projekt entstand aus einer einfachen Idee:

> **„Ich will mehr. Ich will weiter. Also machen wir was Eigenes.“**

Und weil wir offensichtlich nicht genug Projekte gleichzeitig haben:

> **„Mit schlafen ist nicht so viel.“** 😅

---

## 🚧 Status

BuddyFlix befindet sich aktuell in einer sehr frühen Entwicklungsphase.

**Aktueller stabiler Stand: V0.1.0**

V0.1.0 ist ausdrücklich ein **Proof of Concept**. Der wichtigste Meilenstein dieser Version ist: BuddyFlix lässt sich als echtes **QNAP-QPKG** auf einem TS-431P2 installieren und startet dort erfolgreich.

### In V0.1.0 tatsächlich bestätigt

- Installation als QNAP-QPKG über das App Center
- Start auf QNAP TS-431P2 / ARMHF / ARMv7
- BuddyFlix-Weboberfläche erreichbar
- Login funktioniert
- Backend läuft auf Port 8096
- statisches ARMv7-Binary
- erste Dark-Mode-Oberfläche
- grundlegende Server-API vorhanden

### Bereits im Code angelegt, aber in V0.1.0 noch nicht als fertig zu betrachten

- Bibliotheken
- Medien-Scanner
- Filmsuche
- Filmübersicht
- Detailansicht
- Direct Play / HTTP Range Streaming
- Wiedergabefortschritt
- „Weiterschauen“
- Systeminformationen

Diese Bereiche werden aktuell auf dem `develop`-Branch ausgebaut, getestet und erst nach erfolgreichem Praxistest als fertige Funktionen beworben.

> **„Erst testen, dann angeben.“** 😄

---

## 🔑 Standard-Login

Für die aktuelle **V0.1.0** gelten bei einer frischen Installation zunächst diese Zugangsdaten:

```text
Benutzer: admin
Passwort: buddyflix
```

> **Bitte das Standardpasswort nicht für einen öffentlich erreichbaren Server verwenden.**

Der `develop`-Stand ersetzt diese Standardanmeldung bereits durch eine Ersteinrichtung mit eigenem Benutzernamen und Passwort.

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
- automatische Metadaten
- TMDb-Anbindung
- Poster & Backdrops
- Favoriten
- gesehen / ungesehen
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

Installation:

1. QTS öffnen
2. App Center
3. „Manuell installieren“
4. BuddyFlix-QPKG auswählen
5. installieren
6. BuddyFlix im Browser auf Port 8096 öffnen

Die V0.1.0-QPKG-Struktur wurde auf einem TS-431P2 erfolgreich installiert und gestartet.

---

## 🔐 Hinweis

BuddyFlix befindet sich aktuell in einer frühen Entwicklungsphase und ist noch **nicht für den offenen Internetbetrieb gedacht**.

Bitte derzeit bevorzugt nur im internen Netzwerk verwenden.

---

## 🛠️ Technik

Aktuell:

- Go Backend
- statisches ARMv7-Binary
- Weboberfläche
- lokale Persistenz
- QNAP-QPKG-Paketierung
- Direct-Play-orientierte Architektur

Geplant bzw. in Entwicklung:

- echte Verwaltungsoberfläche
- First-Run-Setup
- Medienverwaltung
- TMDb-Identifikation
- Serienstruktur
- SQLite
- ffprobe / FFmpeg Integration
- Multiarch-Builds
- automatisierte Releases

---

## 🧪 Philosophie

BuddyFlix entsteht nicht als akademische Übung, sondern als echte laufende Anwendung auf echter alter Hardware.

Das bedeutet:

- erst testen
- dann schönreden 😄
- keine unnötigen Abhängigkeiten
- keine riesigen Frameworks, wenn es auch klein geht
- lieber schlau arbeiten als rohe CPU-Leistung voraussetzen

Oder inoffiziell:

> **„Wenn das TS-431P2 nicht stirbt, haben wir alles richtig gemacht.“**

---

## 📌 Changelog

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
