package de.buddyflix.firetv

import android.content.Context
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import kotlinx.coroutines.launch

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            BuddyFlixTheme {
                BuddyFlixTvApp()
            }
        }
    }
}

private sealed interface TvScreen {
    data object Connect : TvScreen
    data object Profiles : TvScreen
    data object Library : TvScreen
    data class MovieInfo(val media: MediaEntry) : TvScreen
    data class SeriesInfo(val series: SeriesEntry) : TvScreen
    data class Player(val media: MediaEntry) : TvScreen
}

@Composable
private fun BuddyFlixTvApp() {
    val context = androidx.compose.ui.platform.LocalContext.current
    val prefs = remember { context.getSharedPreferences("buddyflix_tv", Context.MODE_PRIVATE) }

    var baseUrl by remember { mutableStateOf(prefs.getString("server_url", "").orEmpty()) }
    var token by remember { mutableStateOf(prefs.getString("device_token", "").orEmpty()) }
    var profileId by remember { mutableStateOf(prefs.getLong("profile_id", 0L)) }
    var serverName by remember { mutableStateOf(prefs.getString("server_name", "BuddyFlix").orEmpty()) }
    var profiles by remember { mutableStateOf<List<BuddyProfile>>(emptyList()) }
    var library by remember { mutableStateOf<HomeData?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var loading by remember { mutableStateOf(false) }
    var screen by remember {
        mutableStateOf<TvScreen>(
            if (baseUrl.isBlank() || token.isBlank()) TvScreen.Connect else TvScreen.Library
        )
    }

    val scope = rememberCoroutineScope()

    fun api() = BuddyApi(baseUrl, token, profileId)

    suspend fun refreshProfiles() {
        profiles = api().profiles()
        if (profiles.none { it.id == profileId }) profileId = 0L

        if (profiles.size == 1 && profileId == 0L) {
            profileId = profiles.first().id
            prefs.edit().putLong("profile_id", profileId).apply()
        }
    }

    suspend fun refreshLibrary() {
        loading = true
        error = null
        try {
            library = api().home()
        } catch (e: BuddyApiException) {
            if (e.status == 401) {
                token = ""
                profileId = 0L
                prefs.edit().remove("device_token").remove("profile_id").apply()
                screen = TvScreen.Connect
            } else {
                error = e.message ?: "BuddyFlix konnte nicht geladen werden."
            }
        } catch (e: Exception) {
            error = e.message ?: "Verbindung fehlgeschlagen."
        } finally {
            loading = false
        }
    }

    LaunchedEffect(screen, token, profileId) {
        if (token.isBlank()) return@LaunchedEffect

        when (screen) {
            TvScreen.Profiles -> runCatching { refreshProfiles() }
                .onFailure { error = it.message ?: "Profile konnten nicht geladen werden." }

            TvScreen.Library -> {
                runCatching { refreshProfiles() }
                    .onFailure { error = it.message ?: "Profile konnten nicht geladen werden." }

                if (profileId == 0L && profiles.size > 1) {
                    screen = TvScreen.Profiles
                } else if (profileId > 0L) {
                    refreshLibrary()
                }
            }

            else -> Unit
        }
    }

    Box(Modifier.fillMaxSize().background(TvColors.background)) {
        when (val current = screen) {
            TvScreen.Connect -> ConnectScreen(
                initialUrl = baseUrl,
                loading = loading,
                error = error,
                onConnect = { url, username, password ->
                    scope.launch {
                        loading = true
                        error = null
                        try {
                            val normalized = BuddyApi.normalizeBaseUrl(url)
                            val client = BuddyApi(normalized)
                            val info = client.serverInfo()
                            if (info.optString("product") != "BuddyFlix Media Server") {
                                error("Kein BuddyFlix-Server gefunden.")
                            }

                            val login = client.deviceLogin(
                                username,
                                password,
                                "Fire TV " + Build.MODEL
                            )

                            baseUrl = normalized
                            token = login.token
                            serverName = login.serverName

                            prefs.edit()
                                .putString("server_url", normalized)
                                .putString("device_token", token)
                                .putString("server_name", serverName)
                                .apply()

                            profiles = BuddyApi(baseUrl, token).profiles()

                            if (profiles.size == 1) {
                                profileId = profiles.first().id
                                prefs.edit().putLong("profile_id", profileId).apply()
                                screen = TvScreen.Library
                            } else {
                                screen = TvScreen.Profiles
                            }
                        } catch (e: Exception) {
                            error = e.message ?: "Anmeldung fehlgeschlagen."
                        } finally {
                            loading = false
                        }
                    }
                }
            )

            TvScreen.Profiles -> ProfileScreen(
                serverName = serverName,
                profiles = profiles,
                error = error,
                onSelect = { profile ->
                    profileId = profile.id
                    prefs.edit().putLong("profile_id", profile.id).apply()
                    screen = TvScreen.Library
                },
                onChangeServer = {
                    token = ""
                    profileId = 0L
                    prefs.edit().clear().apply()
                    screen = TvScreen.Connect
                }
            )

            TvScreen.Library -> LibraryScreen(
                api = api(),
                serverName = serverName,
                profile = profiles.firstOrNull { it.id == profileId },
                data = library,
                loading = loading,
                error = error,
                onPlay = { screen = TvScreen.Player(it) },
                onMovieInfo = { screen = TvScreen.MovieInfo(it) },
                onSeriesInfo = { screen = TvScreen.SeriesInfo(it) },
                onProfiles = { screen = TvScreen.Profiles },
                onRetry = { scope.launch { refreshLibrary() } },
                onChangeServer = {
                    val client = api()
                    scope.launch { runCatching { client.logoutDevice() } }
                    token = ""
                    profileId = 0L
                    prefs.edit().clear().apply()
                    screen = TvScreen.Connect
                }
            )

            is TvScreen.MovieInfo -> MovieInfoScreen(
                api = api(),
                media = current.media,
                onPlay = { screen = TvScreen.Player(it) },
                onBack = { screen = TvScreen.Library },
                onChanged = { scope.launch { refreshLibrary() } }
            )

            is TvScreen.SeriesInfo -> SeriesInfoScreen(
                api = api(),
                series = current.series,
                onPlay = { screen = TvScreen.Player(it) },
                onBack = { screen = TvScreen.Library }
            )

            is TvScreen.Player -> PlayerScreen(
                api = api(),
                media = current.media,
                onBack = {
                    screen = TvScreen.Library
                    scope.launch { refreshLibrary() }
                }
            )
        }
    }
}
