package de.buddyflix.firetv

import android.content.Context
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.animateFloatAsState
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
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.compose.ui.zIndex
import androidx.media3.common.MediaItem
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.ProgressiveMediaSource
import androidx.media3.ui.PlayerView
import androidx.tv.material3.Button
import androidx.tv.material3.Text
import coil3.compose.AsyncImage
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private val Bg = Color(0xFF05070A)
private val Panel = Color(0xFF0F141C)
private val PanelFocused = Color(0xFF18202B)
private val Accent = Color(0xFFFF6547)
private val AccentSoft = Color(0xFFFFA06C)
private val Muted = Color(0xFF8B96A7)
private val Line = Color(0xFF27303B)

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
    data class Detail(val media: MediaEntry) : TvScreen
    data class SeriesDetail(val series: SeriesEntry) : TvScreen
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
                            val login = client.deviceLogin(user, password, "Fire TV " + Build.MODEL)
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
                api = api(),
                serverName = serverName,
                profile = profiles.firstOrNull { it.id == profileId },
                data = home,
                loading = loading,
                error = error,
                onPlay = { screen = TvScreen.Player(it) },
                onDetail = { screen = TvScreen.Detail(it) },
                onSeries = { screen = TvScreen.SeriesDetail(it) },
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

            is TvScreen.Detail -> MediaDetailScreen(
                api = api(),
                media = current.media,
                onPlay = { screen = TvScreen.Player(it) },
                onBack = { screen = TvScreen.Home },
                onChanged = { scope.launch { loadHome() } }
            )

            is TvScreen.SeriesDetail -> SeriesDetailScreen(
                api = api(),
                series = current.series,
                onPlay = { screen = TvScreen.Player(it) },
                onBack = { screen = TvScreen.Home }
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
    var candidates by remember { mutableStateOf<List<ServerCandidate>>(emptyList()) }
    var discovering by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    LaunchedEffect(Unit) {
        discovering = true
        candidates = ServerDiscovery.discover()
        if (candidates.size == 1 && initialUrl.isBlank()) server = candidates.first().baseUrl
        discovering = false
    }

    Row(
        modifier = Modifier.fillMaxSize().background(
            Brush.radialGradient(
                colors = listOf(Color(0xFF34140D), Bg),
                radius = 1000f
            )
        ).padding(horizontal = 72.dp, vertical = 48.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Column(Modifier.width(430.dp)) {
            Text("BUDDYFLIX", color = Accent, fontWeight = FontWeight.Black, letterSpacing = 5.sp)
            Spacer(Modifier.height(14.dp))
            Text("Dein Kino.\nJetzt auf Fire TV.", color = Color.White, fontSize = 46.sp, lineHeight = 48.sp, fontWeight = FontWeight.Bold)
            Spacer(Modifier.height(18.dp))
            Text(
                "BuddyFlix sucht deinen Server automatisch im Heimnetz. Falls dein Router Broadcasts blockiert, kannst du die Adresse weiterhin manuell eingeben.",
                color = Muted,
                fontSize = 15.sp,
                lineHeight = 23.sp
            )
            Spacer(Modifier.height(24.dp))
            if (discovering) {
                Text("Suche BuddyFlix im Netzwerk …", color = AccentSoft, fontSize = 13.sp)
            } else if (candidates.isNotEmpty()) {
                Text("GEFUNDENE SERVER", color = AccentSoft, fontSize = 10.sp, letterSpacing = 2.sp, fontWeight = FontWeight.Bold)
                Spacer(Modifier.height(10.dp))
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    candidates.take(3).forEach { candidate ->
                        TvFocusBox(
                            modifier = Modifier.fillMaxWidth().height(64.dp),
                            onClick = { server = candidate.baseUrl }
                        ) {
                            Row(Modifier.fillMaxSize().padding(horizontal = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                                Column {
                                    Text(candidate.name, color = Color.White, fontSize = 16.sp, fontWeight = FontWeight.Bold)
                                    Text(candidate.baseUrl, color = Muted, fontSize = 11.sp)
                                }
                                Spacer(Modifier.weight(1f))
                                Text("VERWENDEN", color = AccentSoft, fontSize = 10.sp, fontWeight = FontWeight.Bold)
                            }
                        }
                    }
                }
            } else {
                Button(onClick = {
                    scope.launch {
                        discovering = true
                        candidates = ServerDiscovery.discover()
                        if (candidates.size == 1) server = candidates.first().baseUrl
                        discovering = false
                    }
                }) { Text("Erneut im LAN suchen") }
            }
        }

        Spacer(Modifier.width(72.dp))

        Column(
            Modifier.width(520.dp).background(Color(0xE60C1118), RoundedCornerShape(28.dp))
                .border(1.dp, Line, RoundedCornerShape(28.dp)).padding(30.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            Text("Fire TV verbinden", color = Color.White, fontSize = 28.sp, fontWeight = FontWeight.Bold)
            Text("Nur beim ersten Mal nötig.", color = Muted, fontSize = 13.sp)
            OutlinedTextField(
                value = server,
                onValueChange = { server = it },
                label = { androidx.compose.material3.Text("BuddyFlix-Server") },
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
            if (!error.isNullOrBlank()) Text(error, color = Color(0xFFFFA08D), fontSize = 13.sp)
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
    Column(
        Modifier.fillMaxSize().background(
            Brush.verticalGradient(listOf(Color(0xFF111822), Bg))
        ).padding(56.dp)
    ) {
        Text("BUDDYFLIX · " + serverName.uppercase(), color = Accent, fontSize = 11.sp, letterSpacing = 3.sp, fontWeight = FontWeight.Black)
        Spacer(Modifier.height(10.dp))
        Text("Wer schaut?", color = Color.White, fontSize = 46.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(8.dp))
        Text("Jedes Profil behält seinen eigenen Fortschritt und seine eigene Liste.", color = Muted, fontSize = 15.sp)
        Spacer(Modifier.height(34.dp))

        if (!error.isNullOrBlank()) Text(error, color = Color(0xFFFFA08D))

        LazyRow(horizontalArrangement = Arrangement.spacedBy(20.dp)) {
            items(profiles, key = { it.id }) { profile ->
                TvFocusBox(
                    modifier = Modifier.width(230.dp).height(180.dp),
                    onClick = { onSelect(profile) }
                ) {
                    Column(Modifier.fillMaxSize().padding(22.dp), verticalArrangement = Arrangement.Center) {
                        Text(profile.avatar.ifBlank { "🎬" }, fontSize = 48.sp)
                        Spacer(Modifier.height(10.dp))
                        Text(profile.name, color = Color.White, fontSize = 21.sp, fontWeight = FontWeight.Bold)
                        Text("Profil wählen", color = Muted, fontSize = 11.sp)
                    }
                }
            }
        }

        Spacer(Modifier.weight(1f))
        Button(onClick = onDisconnect) { Text("Anderen Server verwenden") }
    }
}

@Composable
private fun HomeScreen(
    api: BuddyApi,
    serverName: String,
    profile: BuddyProfile?,
    data: HomeData?,
    loading: Boolean,
    error: String?,
    onPlay: (MediaEntry) -> Unit,
    onDetail: (MediaEntry) -> Unit,
    onSeries: (SeriesEntry) -> Unit,
    onProfiles: () -> Unit,
    onRetry: () -> Unit,
    onDisconnect: () -> Unit
) {
    val media = data?.media.orEmpty()
    val movies = media.filter { it.kind != "episode" }
    val continueItems = media.filter { it.progress > 1.0 && it.progress < 95.0 }
        .sortedByDescending { it.progressUpdated }
    val recent = movies.sortedByDescending { it.added }.take(18)
    val favorites = movies.filter { it.favorite }
    var hero by remember(data) { mutableStateOf(continueItems.firstOrNull() ?: recent.firstOrNull() ?: movies.firstOrNull()) }

    Box(Modifier.fillMaxSize()) {
        hero?.let { item ->
            val art = api.imageUrl(item.backdrop.ifBlank { item.poster })
            if (art.isNotBlank()) {
                AsyncImage(
                    model = art,
                    contentDescription = null,
                    modifier = Modifier.fillMaxSize(),
                    contentScale = ContentScale.Crop
                )
            }
        }
        Box(
            Modifier.fillMaxSize().background(
                Brush.horizontalGradient(
                    0f to Color(0xFA05070A),
                    0.50f to Color(0xD805070A),
                    0.82f to Color(0x5005070A),
                    1f to Color(0xCC05070A)
                )
            )
        )
        Box(
            Modifier.fillMaxSize().background(
                Brush.verticalGradient(
                    0f to Color(0x3305070A),
                    0.60f to Color(0x8805070A),
                    1f to Bg
                )
            )
        )

        Column(Modifier.fillMaxSize()) {
            TopBar(serverName, profile, onProfiles, onDisconnect)

            if (loading && data == null) {
                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    Text("Deine Bibliothek wird geladen …", color = Muted, fontSize = 20.sp)
                }
                return@Column
            }

            if (!error.isNullOrBlank()) {
                Row(
                    Modifier.fillMaxWidth().padding(horizontal = 48.dp, vertical = 6.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(error, color = Color(0xFFFFA08D))
                    Spacer(Modifier.width(14.dp))
                    Button(onClick = onRetry) { Text("Nochmal") }
                }
            }

            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(bottom = 54.dp),
                verticalArrangement = Arrangement.spacedBy(28.dp)
            ) {
                hero?.let { item ->
                    item {
                        Hero(api, item, onPlay, onDetail)
                    }
                }
                if (continueItems.isNotEmpty()) {
                    item {
                        MediaRow(
                            api = api,
                            title = "Weiterschauen",
                            kicker = "DEIN FORTSCHRITT",
                            items = continueItems.take(16),
                            wide = true,
                            onFocus = { hero = it },
                            onClick = onDetail
                        )
                    }
                }
                if (recent.isNotEmpty()) {
                    item {
                        MediaRow(
                            api = api,
                            title = "Neu bei BuddyFlix",
                            kicker = "ZULETZT HINZUGEFÜGT",
                            items = recent,
                            onFocus = { hero = it },
                            onClick = onDetail
                        )
                    }
                }
                if (movies.isNotEmpty()) {
                    item {
                        MediaRow(
                            api = api,
                            title = "Filme",
                            kicker = movies.size.toString() + " TITEL",
                            items = movies.take(40),
                            onFocus = { hero = it },
                            onClick = onDetail
                        )
                    }
                }
                if (!data?.series.isNullOrEmpty()) {
                    item {
                        SeriesRow(
                            api = api,
                            title = "Serien",
                            items = data!!.series,
                            onFocus = { it.nextEpisode?.let { ep -> hero = ep.copy(title = it.title, seriesTitle = it.title) } },
                            onClick = onSeries
                        )
                    }
                }
                if (favorites.isNotEmpty()) {
                    item {
                        MediaRow(
                            api = api,
                            title = "Meine Liste",
                            kicker = "FAVORITEN",
                            items = favorites,
                            onFocus = { hero = it },
                            onClick = onDetail
                        )
                    }
                }
                if (movies.isEmpty() && data?.series.isNullOrEmpty()) {
                    item {
                        Text(
                            "Noch keine Medien gefunden.",
                            modifier = Modifier.padding(horizontal = 48.dp),
                            color = Muted,
                            fontSize = 18.sp
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun TopBar(
    serverName: String,
    profile: BuddyProfile?,
    onProfiles: () -> Unit,
    onDisconnect: () -> Unit
) {
    Row(
        Modifier.fillMaxWidth().padding(horizontal = 48.dp, vertical = 18.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Column {
            Text("BUDDYFLIX", color = Accent, fontSize = 13.sp, letterSpacing = 4.sp, fontWeight = FontWeight.Black)
            Text(serverName, color = Color.White.copy(alpha = .72f), fontSize = 11.sp)
        }
        Spacer(Modifier.weight(1f))
        profile?.let {
            Button(onClick = onProfiles) { Text((it.avatar.ifBlank { "🎬" }) + "  " + it.name) }
            Spacer(Modifier.width(10.dp))
        }
        Button(onClick = onDisconnect) { Text("Server") }
    }
}

@Composable
private fun Hero(
    api: BuddyApi,
    media: MediaEntry,
    onPlay: (MediaEntry) -> Unit,
    onDetail: (MediaEntry) -> Unit
) {
    Column(
        Modifier.fillMaxWidth().height(300.dp).padding(horizontal = 48.dp, vertical = 20.dp),
        verticalArrangement = Arrangement.Bottom
    ) {
        Text(if (media.kind == "episode") "WEITERSCHAUEN" else "BUDDYFLIX SPOTLIGHT", color = AccentSoft, fontSize = 10.sp, letterSpacing = 3.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(8.dp))
        Text(media.title, color = Color.White, fontSize = 48.sp, lineHeight = 50.sp, fontWeight = FontWeight.Bold, maxLines = 2, overflow = TextOverflow.Ellipsis)
        Spacer(Modifier.height(8.dp))
        Text(media.subtitle, color = Color.White.copy(alpha = .72f), fontSize = 14.sp)
        if (media.overview.isNotBlank()) {
            Spacer(Modifier.height(10.dp))
            Text(media.overview, color = Color.White.copy(alpha = .78f), fontSize = 14.sp, lineHeight = 20.sp, maxLines = 3, overflow = TextOverflow.Ellipsis, modifier = Modifier.width(650.dp))
        }
        Spacer(Modifier.height(16.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Button(onClick = { onPlay(media) }) {
                Text(if (media.progress > 1.0 && media.progress < 95.0) "▶  Fortsetzen" else "▶  Abspielen")
            }
            Button(onClick = { onDetail(media) }) { Text("Details") }
        }
    }
}

@Composable
private fun MediaRow(
    api: BuddyApi,
    title: String,
    kicker: String,
    items: List<MediaEntry>,
    wide: Boolean = false,
    onFocus: (MediaEntry) -> Unit,
    onClick: (MediaEntry) -> Unit
) {
    Column {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 48.dp),
            verticalAlignment = Alignment.Bottom
        ) {
            Column {
                Text(kicker, color = AccentSoft, fontSize = 9.sp, letterSpacing = 2.sp, fontWeight = FontWeight.Bold)
                Text(title, color = Color.White, fontSize = 27.sp, fontWeight = FontWeight.Bold)
            }
        }
        Spacer(Modifier.height(12.dp))
        LazyRow(
            contentPadding = PaddingValues(horizontal = 48.dp),
            horizontalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            items(items, key = { title + "-" + it.id }) { media ->
                MediaCard(api, media, wide, onFocus, onClick)
            }
        }
    }
}

@Composable
private fun MediaCard(
    api: BuddyApi,
    media: MediaEntry,
    wide: Boolean,
    onFocus: (MediaEntry) -> Unit,
    onClick: (MediaEntry) -> Unit
) {
    var focused by remember { mutableStateOf(false) }
    val scale by animateFloatAsState(if (focused) 1.075f else 1f, label = "cardScale")
    val border by animateColorAsState(if (focused) Color.White else Line, label = "cardBorder")
    val width = if (wide) 278.dp else 168.dp
    val height = if (wide) 158.dp else 252.dp
    val art = api.imageUrl(if (wide) media.backdrop.ifBlank { media.poster } else media.poster.ifBlank { media.backdrop })

    Column(
        modifier = Modifier.width(width).zIndex(if (focused) 2f else 0f)
            .graphicsLayer { scaleX = scale; scaleY = scale }
            .onFocusChanged {
                focused = it.isFocused
                if (it.isFocused) onFocus(media)
            }
            .focusable()
            .clickable { onClick(media) }
    ) {
        Box(
            Modifier.fillMaxWidth().height(height)
                .clip(RoundedCornerShape(18.dp))
                .background(Panel)
                .border(if (focused) 3.dp else 1.dp, border, RoundedCornerShape(18.dp))
        ) {
            if (art.isNotBlank()) {
                AsyncImage(
                    model = art,
                    contentDescription = media.title,
                    modifier = Modifier.fillMaxSize(),
                    contentScale = ContentScale.Crop
                )
            }
            if (art.isBlank()) {
                Box(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(Color(0xFF202938), Panel))))
            }
            Box(
                Modifier.fillMaxSize().background(
                    Brush.verticalGradient(
                        listOf(Color.Transparent, Color.Transparent, Color(0xDD05070A))
                    )
                )
            )
            if (media.progress > 1.0 && media.progress < 95.0) {
                Box(
                    Modifier.fillMaxWidth().height(5.dp).align(Alignment.BottomCenter)
                        .background(Color(0x552A303A))
                ) {
                    Box(
                        Modifier.fillMaxWidth((media.progress / 100.0).toFloat().coerceIn(0f, 1f))
                            .fillMaxHeight().background(Brush.horizontalGradient(listOf(Accent, AccentSoft)))
                    )
                }
            }
        }
        Spacer(Modifier.height(9.dp))
        Text(media.title, color = Color.White, fontSize = 14.sp, fontWeight = FontWeight.Bold, maxLines = 1, overflow = TextOverflow.Ellipsis)
        Text(media.subtitle, color = Muted, fontSize = 10.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

@Composable
private fun SeriesRow(
    api: BuddyApi,
    title: String,
    items: List<SeriesEntry>,
    onFocus: (SeriesEntry) -> Unit,
    onClick: (SeriesEntry) -> Unit
) {
    Column {
        Text(title, modifier = Modifier.padding(horizontal = 48.dp), color = Color.White, fontSize = 27.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(12.dp))
        LazyRow(
            contentPadding = PaddingValues(horizontal = 48.dp),
            horizontalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            items(items, key = { it.key }) { series ->
                var focused by remember { mutableStateOf(false) }
                val scale by animateFloatAsState(if (focused) 1.075f else 1f, label = "seriesScale")
                val art = api.imageUrl(series.poster.ifBlank { series.backdrop })
                Column(
                    Modifier.width(168.dp).zIndex(if (focused) 2f else 0f)
                        .graphicsLayer { scaleX = scale; scaleY = scale }
                        .onFocusChanged { focused = it.isFocused; if (it.isFocused) onFocus(series) }
                        .focusable().clickable { onClick(series) }
                ) {
                    Box(
                        Modifier.fillMaxWidth().height(252.dp).clip(RoundedCornerShape(18.dp))
                            .background(Panel)
                            .border(if (focused) 3.dp else 1.dp, if (focused) Color.White else Line, RoundedCornerShape(18.dp))
                    ) {
                        if (art.isNotBlank()) {
                            AsyncImage(model = art, contentDescription = series.title, modifier = Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
                        }
                        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(Color.Transparent, Color.Transparent, Color(0xDD05070A)))))
                        Text(
                            series.watchedCount.toString() + "/" + series.episodeCount,
                            modifier = Modifier.align(Alignment.BottomEnd).padding(10.dp)
                                .background(Color(0xCC05070A), RoundedCornerShape(99.dp)).padding(horizontal = 8.dp, vertical = 4.dp),
                            color = Color.White,
                            fontSize = 9.sp,
                            fontWeight = FontWeight.Bold
                        )
                    }
                    Spacer(Modifier.height(9.dp))
                    Text(series.title, color = Color.White, fontSize = 14.sp, fontWeight = FontWeight.Bold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Text(series.seasonCount.toString() + " Staffeln · " + series.episodeCount + " Folgen", color = Muted, fontSize = 10.sp)
                }
            }
        }
    }
}

@Composable
private fun MediaDetailScreen(
    api: BuddyApi,
    media: MediaEntry,
    onPlay: (MediaEntry) -> Unit,
    onBack: () -> Unit,
    onChanged: () -> Unit
) {
    BackHandler(onBack = onBack)
    var favorite by remember(media.id) { mutableStateOf(media.favorite) }
    val scope = rememberCoroutineScope()
    val backdrop = api.imageUrl(media.backdrop.ifBlank { media.poster })
    val poster = api.imageUrl(media.poster.ifBlank { media.backdrop })

    Box(Modifier.fillMaxSize().background(Bg)) {
        if (backdrop.isNotBlank()) {
            AsyncImage(model = backdrop, contentDescription = null, modifier = Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
        }
        Box(
            Modifier.fillMaxSize().background(
                Brush.horizontalGradient(listOf(Color(0xFF05070A), Color(0xF005070A), Color(0x8805070A), Color(0xDD05070A)))
            )
        )
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(Color.Transparent, Color(0x5505070A), Bg))))

        Row(Modifier.fillMaxSize().padding(horizontal = 58.dp, vertical = 52.dp), verticalAlignment = Alignment.CenterVertically) {
            Box(
                Modifier.width(235.dp).aspectRatio(2f / 3f).clip(RoundedCornerShape(22.dp))
                    .background(Panel).border(1.dp, Line, RoundedCornerShape(22.dp))
            ) {
                if (poster.isNotBlank()) AsyncImage(model = poster, contentDescription = media.title, modifier = Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
            }
            Spacer(Modifier.width(48.dp))
            Column(Modifier.width(650.dp)) {
                Text(if (media.kind == "episode") media.seriesTitle.uppercase() else "BUDDYFLIX FILM", color = AccentSoft, fontSize = 10.sp, letterSpacing = 2.sp, fontWeight = FontWeight.Bold)
                Spacer(Modifier.height(8.dp))
                Text(media.title, color = Color.White, fontSize = 48.sp, lineHeight = 50.sp, fontWeight = FontWeight.Bold)
                Spacer(Modifier.height(9.dp))
                Text(media.subtitle, color = Color.White.copy(alpha = .70f), fontSize = 14.sp)
                if (media.overview.isNotBlank()) {
                    Spacer(Modifier.height(16.dp))
                    Text(media.overview, color = Color.White.copy(alpha = .78f), fontSize = 15.sp, lineHeight = 22.sp, maxLines = 6, overflow = TextOverflow.Ellipsis)
                }
                if (media.progress > 1.0 && media.progress < 95.0) {
                    Spacer(Modifier.height(18.dp))
                    ProgressBar(media.progress)
                }
                Spacer(Modifier.height(22.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Button(onClick = { onPlay(media) }) {
                        Text(if (media.progress > 1.0 && media.progress < 95.0) "▶  Fortsetzen" else "▶  Abspielen")
                    }
                    Button(onClick = {
                        scope.launch {
                            runCatching { api.mediaAction(media.id, if (favorite) "unfavorite" else "favorite") }
                                .onSuccess { favorite = !favorite; onChanged() }
                        }
                    }) { Text(if (favorite) "♥  In meiner Liste" else "♡  Meine Liste") }
                    Button(onClick = onBack) { Text("Zurück") }
                }
            }
        }
    }
}

@Composable
private fun SeriesDetailScreen(
    api: BuddyApi,
    series: SeriesEntry,
    onPlay: (MediaEntry) -> Unit,
    onBack: () -> Unit
) {
    BackHandler(onBack = onBack)
    var selectedSeason by remember { mutableStateOf(series.nextEpisode?.season ?: series.seasons.firstOrNull()?.number ?: 0) }
    val season = series.seasons.firstOrNull { it.number == selectedSeason } ?: series.seasons.firstOrNull()
    val backdrop = api.imageUrl(series.backdrop.ifBlank { series.poster })

    Box(Modifier.fillMaxSize().background(Bg)) {
        if (backdrop.isNotBlank()) {
            AsyncImage(model = backdrop, contentDescription = null, modifier = Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
        }
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(listOf(Color(0xFA05070A), Color(0xE005070A), Color(0xA005070A)))))
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(Color(0x2205070A), Color(0xCC05070A), Bg))))

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(bottom = 48.dp)
        ) {
            item {
                Column(Modifier.fillMaxWidth().height(330.dp).padding(horizontal = 54.dp, vertical = 36.dp), verticalArrangement = Arrangement.Bottom) {
                    Text("BUDDYFLIX SERIE", color = AccentSoft, fontSize = 10.sp, letterSpacing = 2.sp, fontWeight = FontWeight.Bold)
                    Spacer(Modifier.height(8.dp))
                    Text(series.title, color = Color.White, fontSize = 50.sp, fontWeight = FontWeight.Bold)
                    Spacer(Modifier.height(8.dp))
                    Text(series.seasonCount.toString() + " Staffeln · " + series.episodeCount + " Folgen · " + series.watchedCount + " gesehen", color = Color.White.copy(alpha = .72f), fontSize = 14.sp)
                    Spacer(Modifier.height(16.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        series.nextEpisode?.let { next ->
                            Button(onClick = { onPlay(next) }) {
                                Text(if (next.progress > 1.0 && next.progress < 95.0) "▶  Fortsetzen" else "▶  Nächste Folge")
                            }
                        }
                        Button(onClick = onBack) { Text("Zurück") }
                    }
                }
            }
            item {
                Column {
                    Text("Staffeln", modifier = Modifier.padding(horizontal = 54.dp), color = Color.White, fontSize = 25.sp, fontWeight = FontWeight.Bold)
                    Spacer(Modifier.height(10.dp))
                    LazyRow(
                        contentPadding = PaddingValues(horizontal = 54.dp),
                        horizontalArrangement = Arrangement.spacedBy(9.dp)
                    ) {
                        items(series.seasons, key = { it.number }) { s ->
                            Button(onClick = { selectedSeason = s.number }) {
                                Text(if (s.number == 0) "Extras" else "Staffel " + s.number)
                            }
                        }
                    }
                }
            }
            item {
                Spacer(Modifier.height(24.dp))
                Text(
                    if ((season?.number ?: 0) == 0) "Extras" else "Staffel " + season?.number,
                    modifier = Modifier.padding(horizontal = 54.dp),
                    color = Color.White,
                    fontSize = 25.sp,
                    fontWeight = FontWeight.Bold
                )
                Spacer(Modifier.height(12.dp))
                LazyRow(
                    contentPadding = PaddingValues(horizontal = 54.dp),
                    horizontalArrangement = Arrangement.spacedBy(16.dp)
                ) {
                    items(season?.episodes.orEmpty(), key = { it.id }) { episode ->
                        MediaCard(api, episode, wide = true, onFocus = {}, onClick = { onPlay(it) })
                    }
                }
            }
        }
    }
}

@Composable
private fun ProgressBar(progress: Double) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Box(
            Modifier.width(360.dp).height(6.dp).clip(RoundedCornerShape(99.dp)).background(Color(0x55323B47))
        ) {
            Box(
                Modifier.fillMaxWidth((progress / 100.0).toFloat().coerceIn(0f, 1f)).fillMaxHeight()
                    .background(Brush.horizontalGradient(listOf(Accent, AccentSoft)))
            )
        }
        Spacer(Modifier.width(10.dp))
        Text(progress.toInt().toString() + "%", color = Color.White.copy(alpha = .72f), fontSize = 11.sp)
    }
}

@Composable
private fun TvFocusBox(
    modifier: Modifier = Modifier,
    onClick: () -> Unit,
    content: @Composable ColumnScope.() -> Unit
) {
    var focused by remember { mutableStateOf(false) }
    val scale by animateFloatAsState(if (focused) 1.035f else 1f, label = "focusBox")
    Column(
        modifier = modifier.graphicsLayer { scaleX = scale; scaleY = scale }
            .onFocusChanged { focused = it.isFocused }
            .background(if (focused) PanelFocused else Panel, RoundedCornerShape(18.dp))
            .border(if (focused) 3.dp else 1.dp, if (focused) Color.White else Line, RoundedCornerShape(18.dp))
            .focusable().clickable(onClick = onClick),
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
                    api.saveProgress(media.id, positionMs / 1000.0, durationMs / 1000.0)
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
        modifier = Modifier.fillMaxSize().background(Color.Black).onPreviewKeyEvent {
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
