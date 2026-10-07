package de.buddyflix.firetv

import android.content.Context
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.media3.common.MediaItem
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.ProgressiveMediaSource
import androidx.media3.ui.PlayerView
import androidx.tv.material3.Button
import androidx.tv.material3.Text
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private val Bg = Color(0xFF05070A)
private val Panel = Color(0xFF10151D)
private val PanelFocused = Color(0xFF1A202A)
private val Accent = Color(0xFFFF6B3D)
private val Muted = Color(0xFF8792A2)

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            MaterialTheme {
                androidx.tv.material3.MaterialTheme {
                    BuddyFlixTvApp()
                }
            }
        }
    }
}

private sealed interface TvScreen {
    data object Connecting : TvScreen
    data object ProfilePicker : TvScreen
    data object Home : TvScreen
    data class Player(val media: MediaEntry) : TvScreen
}

@Composable
private fun BuddyFlixTvApp() {
    val context = LocalContext.current
    val prefs = remember { context.getSharedPreferences("buddyflix_tv", Context.MODE_PRIVATE) }
    var baseUrl by remember { mutableStateOf(prefs.getString("server_url", "").orEmpty()) }
    var token by remember { mutableStateOf(prefs.getString("device_token", "").orEmpty()) }
    var profileId by remember { mutableStateOf(prefs.getLong("profile_id", 0L)) }
    var serverName by remember { mutableStateOf(prefs.getString("server_name", "BuddyFlix").orEmpty()) }
    var profiles by remember { mutableStateOf<List<BuddyProfile>>(emptyList()) }
    var home by remember { mutableStateOf<HomeData?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var loading by remember { mutableStateOf(false) }
    var screen by remember {
        mutableStateOf<TvScreen>(if (baseUrl.isBlank() || token.isBlank()) TvScreen.Connecting else TvScreen.Home)
    }
    val scope = rememberCoroutineScope()

    fun api(): BuddyApi = BuddyApi(baseUrl, token, profileId)

    suspend fun loadProfiles(selectAutomatically: Boolean = true) {
        val client = api()
        profiles = client.profiles()
        val savedIsValid = profiles.any { it.id == profileId }
        if (!savedIsValid) profileId = 0L
        if (selectAutomatically && profiles.size == 1) {
            profileId = profiles.first().id
            prefs.edit().putLong("profile_id", profileId).apply()
            screen = TvScreen.Home
        } else if (profileId == 0L) {
            screen = TvScreen.ProfilePicker
        }
    }

    suspend fun loadHome() {
        loading = true
        error = null
        try {
            home = api().home()
        } catch (e: BuddyApiException) {
            if (e.status == 401) {
                token = ""
                profileId = 0L
                prefs.edit().remove("device_token").remove("profile_id").apply()
                screen = TvScreen.Connecting
            } else {
                error = e.message
            }
        } catch (e: Exception) {
            error = e.message ?: "Verbindung fehlgeschlagen"
        } finally {
            loading = false
        }
    }

    LaunchedEffect(screen, profileId, token) {
        if (token.isBlank()) return@LaunchedEffect
        when (screen) {
            TvScreen.ProfilePicker -> runCatching { loadProfiles(false) }.onFailure {
                error = it.message ?: "Profile konnten nicht geladen werden"
            }
            TvScreen.Home -> {
                if (profileId == 0L) {
                    runCatching { loadProfiles() }.onFailure {
                        error = it.message ?: "Profile konnten nicht geladen werden"
                    }
                }
                if (profileId > 0L) loadHome()
            }
            else -> Unit
        }
    }

    Box(Modifier.fillMaxSize().background(Bg)) {
        when (val current = screen) {
            TvScreen.Connecting -> ConnectionScreen(
                initialUrl = baseUrl,
                loading = loading,
                error = error,
                onConnect = { url, user, password ->
                    scope.launch {
                        loading = true
                        error = null
                        try {
                            val normalized = BuddyApi.normalizeBaseUrl(url)
                            val client = BuddyApi(normalized)
                            val info = client.serverInfo()
                            if (info.optString("product") != "BuddyFlix Media Server") {
                                throw IllegalStateException("Das ist kein BuddyFlix-Server.")
                            }
                            val login = client.deviceLogin(
                                user,
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
                            profiles = client.profiles()
                            if (profiles.size == 1) {
                                profileId = profiles.first().id
                                prefs.edit().putLong("profile_id", profileId).apply()
                                screen = TvScreen.Home
                            } else {
                                screen = TvScreen.ProfilePicker
                            }
                        } catch (e: Exception) {
                            error = e.message ?: "Anmeldung fehlgeschlagen"
                        } finally {
                            loading = false
                        }
                    }
                }
            )

            TvScreen.ProfilePicker -> ProfilePicker(
                serverName = serverName,
                profiles = profiles,
                error = error,
                onSelect = { profile ->
                    profileId = profile.id
                    prefs.edit().putLong("profile_id", profile.id).apply()
                    screen = TvScreen.Home
                },
                onDisconnect = {
                    token = ""
                    profileId = 0L
                    prefs.edit().clear().apply()
                    screen = TvScreen.Connecting
                }
            )

            TvScreen.Home -> HomeScreen(
                serverName = serverName,
                profile = profiles.firstOrNull { it.id == profileId },
                data = home,
                loading = loading,
                error = error,
                onPlay = { screen = TvScreen.Player(it) },
                onProfiles = { screen = TvScreen.ProfilePicker },
                onRetry = { scope.launch { loadHome() } },
                onDisconnect = {
                    val client = api()
                    scope.launch { runCatching { client.logoutDevice() } }
                    token = ""
                    profileId = 0L
                    prefs.edit().clear().apply()
                    screen = TvScreen.Connecting
                }
            )

            is TvScreen.Player -> PlayerScreen(
                api = api(),
                media = current.media,
                onBack = {
                    screen = TvScreen.Home
                    scope.launch { loadHome() }
                }
            )
        }
    }
}

@Composable
private fun ConnectionScreen(
    initialUrl: String,
    loading: Boolean,
    error: String?,
    onConnect: (String, String, String) -> Unit
) {
    var server by remember { mutableStateOf(initialUrl.ifBlank { "http://" }) }
    var username by remember { mutableStateOf("admin") }
    var password by remember { mutableStateOf("") }

    Row(
        modifier = Modifier.fillMaxSize().padding(horizontal = 72.dp, vertical = 54.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Column(Modifier.width(460.dp)) {
            Text("BUDDYFLIX", color = Accent, fontWeight = FontWeight.Black, letterSpacing = 4.sp)
            Spacer(Modifier.height(12.dp))
            Text("Dein Kino.
Jetzt auf Fire TV.", color = Color.White, fontSize = 42.sp, lineHeight = 46.sp, fontWeight = FontWeight.Bold)
            Spacer(Modifier.height(18.dp))
            Text(
                "Einmal mit deinem BuddyFlix-Server verbinden. Danach merkt sich der Fernseher sein Gerätetoken.",
                color = Muted,
                fontSize = 16.sp,
                lineHeight = 24.sp
            )
        }

        Spacer(Modifier.width(70.dp))

        Column(
            Modifier.width(520.dp).background(Panel, RoundedCornerShape(24.dp)).padding(28.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            Text("Server verbinden", color = Color.White, fontSize = 26.sp, fontWeight = FontWeight.Bold)
            OutlinedTextField(
                value = server,
                onValueChange = { server = it },
                label = { androidx.compose.material3.Text("Server, z. B. 192.168.1.50:8096") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true
            )
            OutlinedTextField(
                value = username,
                onValueChange = { username = it },
                label = { androidx.compose.material3.Text("Benutzer") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true
            )
            OutlinedTextField(
                value = password,
                onValueChange = { password = it },
                label = { androidx.compose.material3.Text("Passwort") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                visualTransformation = PasswordVisualTransformation(),
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password)
            )
            if (!error.isNullOrBlank()) {
                Text(error, color = Color(0xFFFFA08D), fontSize = 14.sp)
            }
            Button(
                onClick = { if (!loading) onConnect(server, username, password) },
                enabled = !loading && server.isNotBlank() && username.isNotBlank() && password.isNotBlank()
            ) {
                Text(if (loading) "Verbinde …" else "Mit BuddyFlix verbinden")
            }
        }
    }
}

@Composable
private fun ProfilePicker(
    serverName: String,
    profiles: List<BuddyProfile>,
    error: String?,
    onSelect: (BuddyProfile) -> Unit,
    onDisconnect: () -> Unit
) {
    Column(Modifier.fillMaxSize().padding(56.dp)) {
        Text(serverName.uppercase(), color = Accent, fontSize = 12.sp, letterSpacing = 3.sp, fontWeight = FontWeight.Black)
        Spacer(Modifier.height(8.dp))
        Text("Wer schaut?", color = Color.White, fontSize = 40.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(10.dp))
        Text("Jedes Profil hat seinen eigenen Wiedergabefortschritt.", color = Muted, fontSize = 16.sp)
        Spacer(Modifier.height(32.dp))

        if (!error.isNullOrBlank()) Text(error, color = Color(0xFFFFA08D))

        LazyRow(horizontalArrangement = Arrangement.spacedBy(18.dp)) {
            items(profiles, key = { it.id }) { profile ->
                FocusCard(
                    width = 230,
                    height = 170,
                    onClick = { onSelect(profile) }
                ) {
                    Text(profile.avatar.ifBlank { "🎬" }, fontSize = 44.sp)
                    Spacer(Modifier.height(10.dp))
                    Text(profile.name, color = Color.White, fontSize = 20.sp, fontWeight = FontWeight.Bold)
                    Text("Profil wählen", color = Muted, fontSize = 12.sp)
                }
            }
        }

        Spacer(Modifier.weight(1f))
        Button(onClick = onDisconnect) { Text("Anderen Server verwenden") }
    }
}

@Composable
private fun HomeScreen(
    serverName: String,
    profile: BuddyProfile?,
    data: HomeData?,
    loading: Boolean,
    error: String?,
    onPlay: (MediaEntry) -> Unit,
    onProfiles: () -> Unit,
    onRetry: () -> Unit,
    onDisconnect: () -> Unit
) {
    val media = data?.media.orEmpty()
    val movies = media.filter { it.kind != "episode" }
    val continueItems = media
        .filter { it.progress > 1.0 && it.progress < 95.0 }
        .sortedByDescending { it.progressUpdated }
    val seriesItems = data?.series.orEmpty().mapNotNull { it.nextEpisode?.copy(
        title = it.title,
        seriesTitle = it.title
    ) }

    Column(Modifier.fillMaxSize()) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 42.dp, vertical = 22.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column {
                Text("BUDDYFLIX", color = Accent, fontSize = 12.sp, letterSpacing = 3.sp, fontWeight = FontWeight.Black)
                Text(serverName, color = Color.White, fontSize = 24.sp, fontWeight = FontWeight.Bold)
            }
            Spacer(Modifier.weight(1f))
            profile?.let {
                Button(onClick = onProfiles) { Text((it.avatar.ifBlank { "🎬" }) + "  " + it.name) }
                Spacer(Modifier.width(10.dp))
            }
            Button(onClick = onDisconnect) { Text("Trennen") }
        }

        if (loading && data == null) {
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text("Bibliothek wird geladen …", color = Muted, fontSize = 20.sp)
            }
            return@Column
        }

        if (!error.isNullOrBlank()) {
            Row(
                Modifier.fillMaxWidth().padding(horizontal = 42.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(error, color = Color(0xFFFFA08D))
                Spacer(Modifier.width(16.dp))
                Button(onClick = onRetry) { Text("Nochmal") }
            }
        }

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(bottom = 48.dp),
            verticalArrangement = Arrangement.spacedBy(28.dp)
        ) {
            if (continueItems.isNotEmpty()) {
                item {
                    MediaRow("Weiterschauen", continueItems, onPlay)
                }
            }
            if (movies.isNotEmpty()) {
                item {
                    MediaRow("Filme", movies.take(40), onPlay)
                }
            }
            if (seriesItems.isNotEmpty()) {
                item {
                    MediaRow("Serien · nächste Folge", seriesItems, onPlay)
                }
            }
            if (movies.isEmpty() && seriesItems.isEmpty()) {
                item {
                    Text(
                        "Noch keine Medien gefunden.",
                        modifier = Modifier.padding(horizontal = 42.dp),
                        color = Muted,
                        fontSize = 18.sp
                    )
                }
            }
        }
    }
}

@Composable
private fun MediaRow(title: String, items: List<MediaEntry>, onPlay: (MediaEntry) -> Unit) {
    Column {
        Text(
            title,
            modifier = Modifier.padding(horizontal = 42.dp),
            color = Color.White,
            fontSize = 26.sp,
            fontWeight = FontWeight.Bold
        )
        Spacer(Modifier.height(12.dp))
        LazyRow(
            contentPadding = PaddingValues(horizontal = 42.dp),
            horizontalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            items(items, key = { title + "-" + it.id }) { media ->
                FocusCard(
                    width = 250,
                    height = 150,
                    onClick = { onPlay(media) }
                ) {
                    Text(
                        media.title,
                        color = Color.White,
                        fontSize = 18.sp,
                        fontWeight = FontWeight.Bold,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis
                    )
                    Spacer(Modifier.height(8.dp))
                    Text(media.subtitle, color = Muted, fontSize = 12.sp, maxLines = 1)
                    if (media.progress > 1.0 && media.progress < 95.0) {
                        Spacer(Modifier.height(12.dp))
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Box(
                                Modifier.width(150.dp).height(4.dp)
                                    .background(Color(0xFF2A303A), RoundedCornerShape(99.dp))
                            ) {
                                Box(
                                    Modifier.fillMaxWidth((media.progress / 100.0).toFloat().coerceIn(0f, 1f))
                                        .height(4.dp)
                                        .background(Accent, RoundedCornerShape(99.dp))
                                )
                            }
                            Spacer(Modifier.width(8.dp))
                            Text(media.progress.toInt().toString() + "%", color = Muted, fontSize = 11.sp)
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun FocusCard(
    width: Int,
    height: Int,
    onClick: () -> Unit,
    content: @Composable ColumnScope.() -> Unit
) {
    var focused by remember { mutableStateOf(false) }
    Column(
        modifier = Modifier
            .width(width.dp)
            .height(height.dp)
            .onFocusChanged { focused = it.isFocused }
            .background(if (focused) PanelFocused else Panel, RoundedCornerShape(20.dp))
            .border(
                width = if (focused) 3.dp else 1.dp,
                color = if (focused) Color.White else Color(0xFF252C36),
                shape = RoundedCornerShape(20.dp)
            )
            .clickable(onClick = onClick)
            .focusable()
            .padding(18.dp),
        verticalArrangement = Arrangement.Center,
        content = content
    )
}

@Composable
private fun PlayerScreen(
    api: BuddyApi,
    media: MediaEntry,
    onBack: () -> Unit
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()

    val player = remember(media.id) {
        val dataSourceFactory = DefaultHttpDataSource.Factory()
            .setDefaultRequestProperties(api.streamHeaders())
        val source = ProgressiveMediaSource.Factory(dataSourceFactory)
            .createMediaSource(MediaItem.fromUri(api.streamUrl(media.id)))
        ExoPlayer.Builder(context).build().apply {
            setMediaSource(source)
            prepare()
            if (media.position > 20.0 && media.progress < 95.0) {
                seekTo((media.position * 1000.0).toLong())
            }
            playWhenReady = true
        }
    }

    fun save() {
        val durationMs = player.duration
        val positionMs = player.currentPosition
        if (durationMs > 0) {
            scope.launch {
                runCatching {
                    api.saveProgress(
                        media.id,
                        positionMs / 1000.0,
                        durationMs / 1000.0
                    )
                }
            }
        }
    }

    BackHandler {
        save()
        onBack()
    }

    LaunchedEffect(player) {
        while (true) {
            delay(10_000)
            save()
        }
    }

    DisposableEffect(player) {
        onDispose {
            save()
            player.release()
        }
    }

    AndroidView(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black)
            .onPreviewKeyEvent {
                if (it.key == Key.Back) {
                    save()
                    onBack()
                    true
                } else false
            },
        factory = { ctx ->
            PlayerView(ctx).apply {
                useController = true
                controllerAutoShow = true
                this.player = player
                requestFocus()
            }
        },
        update = { it.player = player }
    )
}
