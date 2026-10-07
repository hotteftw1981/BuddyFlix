# 🎬 BuddyFlix

> **Schlanker Media Server für Hardware, auf der andere längst aufgegeben haben.**

BuddyFlix ist ein eigener, ressourcenschonender Media Server mit Fokus auf **ARMHF / ARMv7**, Direct Play und ältere NAS-Hardware.  
Entstanden ist das Projekt auf einem **QNAP TS-431P2 mit AnnapurnaLabs AL-314 und 8 GB RAM**.

> **„Wenn Jellyfin auf ARMHF stehen bleibt, bauen wir halt selbst weiter.“**

---

## 🚧 Projektstatus

| Branch | Status | Zweck |
| --- | --- | --- |
| **`main`** | 🟢 stabiler Referenzstand | bestätigte Releases |
| **`develop`** | 🧪 aktive Entwicklung | neue Funktionen, Testpakete und Experimente |

**Stable:** `V0.1.0`  
**Development:** `V0.1.1-dev`

> ⚠️ Der Entwicklungsbranch enthält Funktionen, die bereits implementiert sind, aber noch nicht zwingend auf jeder Zielhardware vollständig getestet wurden.

BuddyFlix ist weiterhin **Early Development** und aktuell primär für den Einsatz im **lokalen Netzwerk** gedacht.

---

## ✨ Was BuddyFlix heute schon kann

### 🎞️ Bibliothek & Medien

- Film- und Serienbibliotheken
- Bibliotheksscans auf NAS-Pfaden
- Erkennung fehlender/verwaister Medien
- Ausschluss typischer QNAP-/NAS-Systemordner
- Filme, Serien, Staffeln und Episoden
- Medieneditor
- gesehen / ungesehen
- Favoriten / „Meine Liste“
- Wiedergabefortschritt
- „Weiterschauen“
- nächste Serienfolge
- profilbezogener Fortschritt

### 🧠 Metadaten

- TheTVDB als primärer Metadatenprovider
- automatische Zuordnung eindeutiger Treffer
- Prüfliste für unsichere Zuordnungen
- manuelle Auswahl eines TheTVDB-Treffers
- persistente externe IDs
- Speicherung der Herkunft übernommener Metadatenfelder
- provider-neutrale Metadata Engine als Basis für weitere Quellen

**Metadata provided by [TheTVDB](https://thetvdb.com). Please consider adding missing information or subscribing.**

### 👥 Profile

- mehrere Profile
- eigener Wiedergabefortschritt pro Profil
- eigene Favoriten pro Profil
- Profilwahl im Web und im experimentellen TV-Client

### ▶️ Wiedergabe

BuddyFlix verfolgt bewusst eine **Direct-Play-first**-Strategie:

```text
Direct Play
    ↓
Direct Stream / Remux
    ↓
Audio Transcoding
    ↓
Video Transcoding
```

Aktuell liegt der Schwerpunkt klar auf **Direct Play** und geringer Last auf schwacher NAS-Hardware.

> **„Wir versuchen nicht, auf vier Cortex-A15-Kernen Netflix' Rechenzentrum nachzuspielen.“**

---

## 🖥️ Weboberfläche

Der Webclient befindet sich ebenfalls aktiv in Entwicklung und enthält unter anderem:

- Dark Mode
- Startseite mit Hero-Bereich
- Film- und Serienübersichten
- CineWall
- Weiterschauen
- Serienfortschritt
- Staffeln und Episoden
- Film-/Seriendetails
- Favoriten
- Profilwahl
- Administration
- TV-/Tastatur-Fokusgrundlagen

Das aktuelle Design im `develop`-Branch wird laufend überarbeitet und ist noch nicht als finale Oberfläche zu verstehen.

---

## 🔥📺 Fire TV — experimentell

BuddyFlix besitzt inzwischen einen **nativen Fire-TV-Client** auf Basis von:

- Kotlin
- Jetpack Compose
- Compose for TV
- AndroidX Media3 / ExoPlayer

Der Client ist **kein WebView**.

### Bereits vorhanden

- Verbindung zu einem BuddyFlix-Server
- dauerhafte Geräte-Tokens
- Profilwahl
- Filme und Serien
- Weiterschauen
- Poster und Backdrops
- Serien-/Staffel-/Folgenansichten
- D-Pad-/Fernbedienungsnavigation
- Fortschrittssynchronisation
- Direct-Play-Player
- LAN-Servererkennung per Broadcast mit Subnetz-Scan als Fallback

### Aktueller Zustand

> ⚠️ **Der Fire-TV-Client ist ausdrücklich experimentell.**

Die Oberfläche und Fokusnavigation werden derzeit stark überarbeitet.  
Im aktuellen Entwicklungsstand besteht außerdem eine bekannte **Playback-Regression**, bei der Filme auf dem Fire TV nicht zuverlässig starten.

Die App ist damit momentan **Testclient**, noch kein fertiger Alltags-Client.

### Installation per ADB

Beispiel:

```powershell
adb connect FIRE_TV_IP:5555
adb install -r BuddyFlix-FireTV.apk
```

Geplant sind unter anderem weitere Verbesserungen an:

- Fokus- und Scrollverhalten
- Player-Stabilität
- Tonspur- und Untertitelwahl
- serverseitiger Geräteverwaltung
- TV-optimierter Detailansicht
- allgemeinem UI-Polish

---

## 📦 QNAP / ARMHF

BuddyFlix wird als **QPKG** für QNAP gebaut.

Bestätigte Referenzhardware:

- **QNAP TS-431P2**
- AnnapurnaLabs Alpine AL-314
- 4 × 1,7 GHz Cortex-A15
- ARMv7 / ARMHF
- 8 GB RAM

### Installation

1. QTS öffnen
2. **App Center**
3. **Manuell installieren**
4. BuddyFlix-`.qpkg` auswählen
5. installieren
6. BuddyFlix im Browser öffnen

Standardport:

```text
8096
```

### Development Builds

Im `develop`-Branch werden Test-QPKGs nur noch **gezielt** gebaut:

- manuell über GitHub Actions
- oder per Commit mit `[qpkg]`

Normale Entwicklungs-Commits führen nur die günstigen Checks aus.

Fire-TV-APKs werden entsprechend nur gezielt per `[firetv]` oder manuell gebaut.

---

## 🔑 Anmeldung

### Stable `V0.1.0`

```text
Benutzer: admin
Passwort: buddyflix
```

> Das Standardpasswort sollte niemals für einen öffentlich erreichbaren Server verwendet werden.

### `develop`

Bei einer frischen Installation führt BuddyFlix durch die Ersteinrichtung.  
Dort werden **Servername, Admin-Benutzer und Passwort** festgelegt.

Native Clients können danach ein eigenes **Geräte-Token** erhalten. Das Admin-Passwort muss dadurch nicht dauerhaft auf dem Client gespeichert werden.

Eine Änderung des Admin-Passworts widerruft bestehende Geräte-Tokens.

---

## 🔐 Sicherheit

BuddyFlix befindet sich noch in einer frühen Entwicklungsphase.

Aktuell empfohlen:

- Nutzung im internen Netzwerk
- kein offener Internetbetrieb
- eigenes Admin-Passwort
- regelmäßige Updates bei Testinstallationen

Die Client-API unterstützt sowohl Browser-Sessions als auch persistente Geräte-Tokens für native Apps.

---

## 🛠️ Technik

### Backend

- Go
- statisches ARMv7-Binary
- lokale Persistenz
- HTTP Range Streaming
- profilbezogene Fortschrittsdaten
- Geräte-Token-Authentifizierung
- Discovery-Endpunkte
- LAN-Discovery für native Clients

### Frontend

- eigener Webclient
- Dark-Mode-first
- responsive Oberfläche
- Film- und Serienansichten
- Profilunterstützung

### Build & CI

- GitHub Actions
- Go-Tests
- JavaScript-Syntaxprüfung
- ARMHF-QPKG-Build
- nativer Fire-TV-APK-Build
- kurzlebige Dev-Artefakte
- kostenbewusste Builds nur bei Bedarf

---

## 🧭 Roadmap

BuddyFlix soll **kein 1:1-Jellyfin-Klon** werden.

Der Fokus bleibt auf einem kleinen, kontrollierbaren Stack für ältere Hardware.

### Nächste Schwerpunkte

- Fire-TV-Player wieder stabilisieren
- Fire-TV-UI und Fokusnavigation sauber neu ausarbeiten
- Metadaten-Matching weiter verbessern
- Serienlogik und Episodenhandling verfeinern
- Untertitel
- Tonspurwahl
- Direct Stream / Remux
- Audio-Transcoding
- Video-Transcoding nur als letzte Option
- weitere Plattformen später nur dann, wenn der Kern stabil ist

---

## 🧪 Entwicklungsprinzipien

BuddyFlix entsteht auf echter Hardware und nicht nur in einer Entwicklungsumgebung.

Darum gelten ein paar einfache Regeln:

- **erst testen, dann behaupten**
- Direct Play vor Transcoding
- geringe CPU- und RAM-Last
- keine unnötig schweren Abhängigkeiten
- Stable README = bestätigte Funktionen
- Development README = transparenter Entwicklungsstand
- keine künstlich aufgeblähte Featureliste

> **„Wenn das TS-431P2 nicht stirbt, haben wir alles richtig gemacht.“** 😄

---

## 📌 Versionsstand

### `V0.1.1-dev`

Aktiver Entwicklungszweig mit unter anderem:

- First-Run-Setup
- erweiterter Administration
- Bibliotheksverwaltung
- robusteren NAS-Scans
- Filme + Serien + Staffeln + Episoden
- Weiterschauen
- Profile
- profilbezogenem Fortschritt
- Favoriten
- TheTVDB-Integration
- provider-neutraler Metadata Engine
- externen IDs und Quellenherkunft
- Geräte-Token-Authentifizierung
- nativer Fire-TV-App
- LAN-Servererkennung
- kostenoptimierter CI

### `V0.1.0`

Erster bestätigter QNAP-Proof-of-Concept:

- ARMHF-/ARMv7-Serverbasis
- Login
- Dark-Mode-Weboberfläche
- Installation über das QNAP App Center
- Start auf einem QNAP TS-431P2
- Backend auf Port 8096

---

## ❤️ Projekt

BuddyFlix ist ein öffentliches Hobby-/Entwicklungsprojekt rund um die Frage:

> **„Wie weit kommt man mit einem alten ARM-NAS, wenn man einfach nicht akzeptiert, dass es angeblich zu alt ist?“**

Ziemlich weit, offenbar. 😄🍿
