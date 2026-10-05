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

**Aktueller Stand: V0.1.0**

Die erste Version läuft bereits als echtes **QNAP-QPKG** auf einem TS-431P2 und kann direkt über das QNAP App Center manuell installiert werden.

### Bereits vorhanden

- Login
- moderne Dark-Mode-Weboberfläche
- Bibliotheken
- Medien-Scanner
- Filmsuche
- Filmübersicht
- Detailansicht
- Direct Play
- HTTP Range Requests
- Wiedergabefortschritt
- „Weiterschauen“
- Systemstatus
- QNAP-QPKG für ARMHF / ARMv7
- ARMHF-first Architektur

---

## 🔑 Standard-Login

Für die aktuelle **V0.1.0** gelten bei einer frischen Installation zunächst diese Zugangsdaten:

```text
Benutzer: admin
Passwort: buddyflix
```

> **Bitte das Standardpasswort nicht für einen öffentlich erreichbaren Server verwenden.**

Ein erzwungener Passwortwechsel beim ersten Start ist bereits als sinnvoller nächster Schritt vorgesehen.

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
- PWA
- Android-/Android-TV-Client
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

Entwicklung und erster Produktivtest:

- **QNAP TS-431P2**
- AnnapurnaLabs Alpine AL-314
- 4 × 1,7 GHz Cortex-A15
- ARMv7 / ARMHF
- 8 GB RAM

BuddyFlix wird so entwickelt, dass dieses Gerät als Mindestniveau gilt.

Wenn es dort flott läuft, läuft es auf neuerer Hardware erst recht.

---

## 📦 QNAP

Die aktuelle QNAP-Version wird als **.qpkg** bereitgestellt.

Installation:

1. QTS öffnen
2. App Center
3. „Manuell installieren“
4. BuddyFlix-QPKG auswählen
5. installieren
6. BuddyFlix im Browser öffnen

Standard-Port:

```
8096
```

Aktuell ist die erste QPKG-Linie speziell auf ARMHF-/ARMv7-QNAPs ausgelegt.

---

## 🔐 Hinweis

BuddyFlix befindet sich aktuell in einer frühen Entwicklungsphase und ist noch **nicht für den offenen Internetbetrieb gedacht**.

Bitte derzeit bevorzugt nur im internen Netzwerk verwenden.

---

## 🛠️ Technik

Aktuell:

- Go Backend
- statisches ARMv7-Binary
- eingebettete Weboberfläche
- lokale Persistenz
- QNAP-QPKG-Paketierung
- Direct-Play-orientierter Streaming-Stack

Geplant:

- SQLite
- ffprobe / FFmpeg Integration
- erweiterte Metadaten-Provider
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

Erster lauffähiger BuddyFlix-Stand.

- ARMHF-/ARMv7-Serverbasis
- Login
- Dark-Mode-Webinterface
- Bibliotheksverwaltung
- Medien-Scanner
- Filmsuche
- Filmansicht
- Direct Play
- Range-Streaming
- Wiedergabefortschritt
- „Weiterschauen“
- Systemstatus
- funktionierendes QNAP-QPKG für TS-431P2
- QNAP App Center Installation erfolgreich getestet

---

## ❤️ Projektstatus

**Experimental / Early Development**

Das Projekt ist öffentlich, weil wir finden:

> **„Wenn wir schon so einen Unsinn bauen, dann wenigstens öffentlich.“** 😄

Beiträge, Tests, Ideen und Bugreports sind willkommen.
