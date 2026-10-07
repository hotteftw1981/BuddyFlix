# BuddyFlix Fire TV

Native TV client for BuddyFlix Media Server.

## Current development scope

The first test build is deliberately small but end-to-end:

- connect to a BuddyFlix server by LAN URL
- one-time admin login that creates a persistent device token
- profile selection
- movie and series rows
- continue-watching row
- direct playback through AndroidX Media3 / ExoPlayer
- HTTP range streaming from BuddyFlix
- periodic playback-progress sync back to the selected BuddyFlix profile
- Fire TV remote / D-pad friendly focus states
- cleartext LAN HTTP support for local NAS installs

## Platform

- Kotlin
- Jetpack Compose
- Compose for TV
- AndroidX Media3
- minSdk 25 (Fire OS 6 / Android 7.1 and newer Android-based Fire TV devices)

Amazon has started shipping some 2026 Fire TV hardware with Vega OS. This Android APK is for Android/Fire OS based Fire TV devices; Vega OS will need its own client later.

## Build

The repository intentionally does not build the APK on every commit.

A Fire TV APK is built when:

- the workflow is launched manually, or
- a commit that touches the Fire TV client contains `[firetv]`

Artifacts are retained only briefly for development testing.

## First test

1. Install the APK by ADB / sideloading.
2. Enter the BuddyFlix server LAN address, e.g. `192.168.1.50:8096`.
3. Log in once with the BuddyFlix admin credentials.
4. Choose a profile.
5. Start a movie or the next episode of a series.

The Fire TV stores only the returned device token, not the admin password.
