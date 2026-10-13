package de.buddyflix.firetv

import androidx.activity.compose.BackHandler
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
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
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
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import coil3.compose.AsyncImage
import kotlinx.coroutines.launch

private enum class LibrarySection(val label: String) {
    HOME("Start"),
    MOVIES("Filme"),
    SERIES("Serien"),
    LIST("Meine Liste")
}

@Composable
fun ConnectScreen(
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
        if (candidates.size == 1 && initialUrl.isBlank()) {
            server = candidates.first().baseUrl
        }
        discovering = false
    }

    val fieldColors = OutlinedTextFieldDefaults.colors(
        focusedTextColor = TvColors.text,
        unfocusedTextColor = TvColors.text,
        focusedBorderColor = TvColors.focus,
        unfocusedBorderColor = TvColors.line,
        focusedLabelColor = TvColors.text,
        unfocusedLabelColor = TvColors.textMuted,
        cursorColor = TvColors.focus,
        focusedContainerColor = TvColors.surfaceSoft,
        unfocusedContainerColor = TvColors.surfaceSoft
    )

    Row(
        Modifier.fillMaxSize()
            .background(TvColors.background)
            .padding(horizontal = 64.dp, vertical = 48.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Column(Modifier.width(420.dp)) {
            Text(
                "BUDDYFLIX",
                color = TvColors.brand,
                fontSize = 14.sp,
                fontWeight = FontWeight.Black,
                letterSpacing = 4.sp
            )
            Spacer(Modifier.height(14.dp))
            Text(
                "Dein Kino.\nOhne Schnickschnack.",
                color = TvColors.text,
                fontSize = 42.sp,
                lineHeight = 45.sp,
                fontWeight = FontWeight.Bold
            )
            Spacer(Modifier.height(16.dp))
            Text(
                "BuddyFlix verbindet sich direkt mit deinem Server im Heimnetz.",
                color = TvColors.textMuted,
                fontSize = 14.sp,
                lineHeight = 21.sp
            )
            Spacer(Modifier.height(28.dp))

            Text(
                "SERVER",
                color = TvColors.textDim,
                fontSize = 9.sp,
                fontWeight = FontWeight.Bold,
                letterSpacing = 2.sp
            )
            Spacer(Modifier.height(10.dp))

            Box(Modifier.width(380.dp).height(154.dp)) {
                when {
                    discovering -> StatusPanel(
                        title = "Suche im Heimnetz …",
                        body = "Broadcast und lokaler Netzscan laufen."
                    )

                    candidates.isNotEmpty() -> {
                        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                            candidates.take(2).forEach { candidate ->
                                FocusSurface(
                                    modifier = Modifier.fillMaxWidth().height(72.dp),
                                    onClick = { server = candidate.baseUrl }
                                ) {
                                    Row(
                                        Modifier.fillMaxSize().padding(horizontal = 16.dp),
                                        verticalAlignment = Alignment.CenterVertically
                                    ) {
                                        Column {
                                            Text(candidate.name, color = TvColors.text, fontSize = 14.sp, fontWeight = FontWeight.Bold)
                                            Text(candidate.baseUrl, color = TvColors.textMuted, fontSize = 10.sp)
                                        }
                                        Spacer(Modifier.weight(1f))
                                        Text("WÄHLEN", color = TvColors.textMuted, fontSize = 9.sp, fontWeight = FontWeight.Bold)
                                    }
                                }
                            }
                        }
                    }

                    else -> StatusPanel(
                        title = "Kein Server automatisch gefunden",
                        body = "Du kannst die Adresse rechts direkt eingeben.",
                        action = "Erneut suchen",
                        onAction = {
                            scope.launch {
                                discovering = true
                                candidates = ServerDiscovery.discover()
                                if (candidates.size == 1) server = candidates.first().baseUrl
                                discovering = false
                            }
                        }
                    )
                }
            }
        }

        Spacer(Modifier.width(70.dp))

        Column(
            Modifier.width(500.dp)
                .background(TvColors.surface, RoundedCornerShape(24.dp))
                .border(1.dp, TvColors.line, RoundedCornerShape(24.dp))
                .padding(28.dp)
        ) {
            Text("Fire TV verbinden", color = TvColors.text, fontSize = 26.sp, fontWeight = FontWeight.Bold)
            Spacer(Modifier.height(5.dp))
            Text("Einmal anmelden, danach bleibt dieser Cube verbunden.", color = TvColors.textMuted, fontSize = 12.sp)
            Spacer(Modifier.height(20.dp))

            OutlinedTextField(
                value = server,
                onValueChange = { server = it },
                modifier = Modifier.fillMaxWidth(),
                label = { Text("Serveradresse") },
                placeholder = { Text("192.168.1.50:8096") },
                singleLine = true,
                colors = fieldColors,
                shape = RoundedCornerShape(12.dp)
            )
            Spacer(Modifier.height(11.dp))
            OutlinedTextField(
                value = username,
                onValueChange = { username = it },
                modifier = Modifier.fillMaxWidth(),
                label = { Text("Benutzer") },
                singleLine = true,
                colors = fieldColors,
                shape = RoundedCornerShape(12.dp)
            )
            Spacer(Modifier.height(11.dp))
            OutlinedTextField(
                value = password,
                onValueChange = { password = it },
                modifier = Modifier.fillMaxWidth(),
                label = { Text("Passwort") },
                singleLine = true,
                colors = fieldColors,
                shape = RoundedCornerShape(12.dp),
                visualTransformation = PasswordVisualTransformation(),
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password)
            )

            Box(Modifier.fillMaxWidth().height(44.dp).padding(top = 8.dp)) {
                if (!error.isNullOrBlank()) {
                    Text(error, color = TvColors.error, fontSize = 11.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
                } else {
                    Text("Das Passwort wird nicht dauerhaft gespeichert.", color = TvColors.textDim, fontSize = 10.sp)
                }
            }

            ActionButton(
                text = if (loading) "Verbinde …" else "Verbinden",
                enabled = !loading && server.isNotBlank() && username.isNotBlank() && password.isNotBlank(),
                onClick = { if (!loading) onConnect(server, username, password) }
            )
        }
    }
}

@Composable
private fun StatusPanel(
    title: String,
    body: String,
    action: String? = null,
    onAction: (() -> Unit)? = null
) {
    Column(
        Modifier.fillMaxSize()
            .background(TvColors.surfaceSoft, RoundedCornerShape(16.dp))
            .border(1.dp, TvColors.line, RoundedCornerShape(16.dp))
            .padding(18.dp),
        verticalArrangement = Arrangement.Center
    ) {
        Text(title, color = TvColors.text, fontSize = 14.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(5.dp))
        Text(body, color = TvColors.textMuted, fontSize = 11.sp, lineHeight = 16.sp)
        if (action != null && onAction != null) {
            Spacer(Modifier.height(10.dp))
            ActionButton(text = action, compact = true, onClick = onAction)
        }
    }
}

@Composable
fun ProfileScreen(
    serverName: String,
    profiles: List<BuddyProfile>,
    error: String?,
    onSelect: (BuddyProfile) -> Unit,
    onChangeServer: () -> Unit
) {
    Column(
        Modifier.fillMaxSize()
            .background(TvColors.background)
            .padding(56.dp)
    ) {
        Text("BUDDYFLIX", color = TvColors.brand, fontSize = 12.sp, fontWeight = FontWeight.Black, letterSpacing = 4.sp)
        Spacer(Modifier.height(6.dp))
        Text(serverName, color = TvColors.textDim, fontSize = 11.sp)
        Spacer(Modifier.height(26.dp))
        Text("Wer schaut?", color = TvColors.text, fontSize = 40.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(8.dp))
        Text("Fortschritt und Favoriten bleiben pro Profil getrennt.", color = TvColors.textMuted, fontSize = 14.sp)
        Spacer(Modifier.height(30.dp))

        if (!error.isNullOrBlank()) {
            Text(error, color = TvColors.error, fontSize = 12.sp)
            Spacer(Modifier.height(12.dp))
        }

        LazyRow(horizontalArrangement = Arrangement.spacedBy(14.dp)) {
            items(profiles, key = { it.id }) { profile ->
                FocusSurface(
                    modifier = Modifier.width(210.dp).height(154.dp),
                    onClick = { onSelect(profile) }
                ) {
                    Column(
                        Modifier.fillMaxSize().padding(20.dp),
                        verticalArrangement = Arrangement.Center
                    ) {
                        Text(profile.avatar.ifBlank { "🎬" }, fontSize = 40.sp)
                        Spacer(Modifier.height(8.dp))
                        Text(profile.name, color = TvColors.text, fontSize = 18.sp, fontWeight = FontWeight.Bold)
                        Spacer(Modifier.height(2.dp))
                        Text("Profil öffnen", color = TvColors.textMuted, fontSize = 10.sp)
                    }
                }
            }
        }

        Spacer(Modifier.weight(1f))
        ActionButton(text = "Anderen Server verwenden", compact = true, onClick = onChangeServer)
    }
}

@Composable
fun LibraryScreen(
    api: BuddyApi,
    serverName: String,
    profile: BuddyProfile?,
    data: HomeData?,
    loading: Boolean,
    error: String?,
    onPlay: (MediaEntry) -> Unit,
    onMovieInfo: (MediaEntry) -> Unit,
    onSeriesInfo: (SeriesEntry) -> Unit,
    onProfiles: () -> Unit,
    onRetry: () -> Unit,
    onChangeServer: () -> Unit
) {
    var section by remember { mutableStateOf(LibrarySection.HOME) }

    val media = data?.media.orEmpty()
    val movies = media.filter { it.kind != "episode" }
    val continueItems = media
        .filter { it.progress > 1.0 && it.progress < 95.0 }
        .sortedByDescending { it.progressUpdated }
    val recent = movies.sortedByDescending { it.added }
    val favorites = movies.filter { it.favorite }
    val hero = continueItems.firstOrNull() ?: recent.firstOrNull() ?: movies.firstOrNull()

    Row(Modifier.fillMaxSize().background(TvColors.background)) {
        NavigationRail(
            active = section,
            profile = profile,
            onSelect = { section = it },
            onProfiles = onProfiles,
            onChangeServer = onChangeServer
        )

        Box(
            Modifier.weight(1f).fillMaxHeight()
                .border(width = 0.dp, color = Color.Transparent)
        ) {
            when {
                loading && data == null -> {
                    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        Text("Bibliothek wird geladen …", color = TvColors.textMuted, fontSize = 17.sp)
                    }
                }

                !error.isNullOrBlank() -> {
                    Column(
                        Modifier.fillMaxSize().padding(48.dp),
                        verticalArrangement = Arrangement.Center
                    ) {
                        Text("BuddyFlix konnte nicht geladen werden.", color = TvColors.text, fontSize = 24.sp, fontWeight = FontWeight.Bold)
                        Spacer(Modifier.height(8.dp))
                        Text(error, color = TvColors.error, fontSize = 12.sp)
                        Spacer(Modifier.height(16.dp))
                        ActionButton(text = "Nochmal versuchen", onClick = onRetry)
                    }
                }

                else -> when (section) {
                    LibrarySection.HOME -> HomeSection(
                        api = api,
                        serverName = serverName,
                        hero = hero,
                        continueItems = continueItems,
                        recent = recent.take(12),
                        onPlay = onPlay,
                        onInfo = onMovieInfo
                    )

                    LibrarySection.MOVIES -> MovieGridSection(
                        api = api,
                        title = "Filme",
                        subtitle = movies.size.toString() + " Titel",
                        movies = movies,
                        onPlay = onPlay
                    )

                    LibrarySection.SERIES -> SeriesGridSection(
                        api = api,
                        series = data?.series.orEmpty(),
                        onOpen = onSeriesInfo
                    )

                    LibrarySection.LIST -> MovieGridSection(
                        api = api,
                        title = "Meine Liste",
                        subtitle = favorites.size.toString() + " Favoriten",
                        movies = favorites,
                        onPlay = onPlay
                    )
                }
            }
        }
    }
}

@Composable
private fun NavigationRail(
    active: LibrarySection,
    profile: BuddyProfile?,
    onSelect: (LibrarySection) -> Unit,
    onProfiles: () -> Unit,
    onChangeServer: () -> Unit
) {
    Column(
        Modifier.width(122.dp).fillMaxHeight()
            .background(Color(0xFF0B0C0F))
            .border(1.dp, TvColors.line)
            .padding(horizontal = 12.dp, vertical = 22.dp)
    ) {
        Text(
            "B",
            modifier = Modifier.align(Alignment.CenterHorizontally),
            color = TvColors.brand,
            fontSize = 28.sp,
            fontWeight = FontWeight.Black
        )

        Spacer(Modifier.height(34.dp))

        LibrarySection.entries.forEach { item ->
            NavButton(
                text = item.label,
                selected = active == item,
                onClick = { onSelect(item) }
            )
            Spacer(Modifier.height(8.dp))
        }

        Spacer(Modifier.weight(1f))

        profile?.let {
            NavButton(
                text = it.name,
                selected = false,
                onClick = onProfiles
            )
            Spacer(Modifier.height(8.dp))
        }

        NavButton(
            text = "Server",
            selected = false,
            onClick = onChangeServer
        )
    }
}

@Composable
private fun NavButton(
    text: String,
    selected: Boolean,
    onClick: () -> Unit
) {
    var focused by remember { mutableStateOf(false) }

    Box(
        Modifier.fillMaxWidth().height(48.dp)
            .onFocusChanged { focused = it.isFocused }
            .background(
                when {
                    focused -> TvColors.surfaceRaised
                    selected -> TvColors.surface
                    else -> Color.Transparent
                },
                RoundedCornerShape(10.dp)
            )
            .border(
                width = if (focused) 2.dp else 1.dp,
                color = when {
                    focused -> TvColors.focus
                    selected -> TvColors.line
                    else -> Color.Transparent
                },
                shape = RoundedCornerShape(10.dp)
            )
            .focusable()
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text,
            color = if (selected || focused) TvColors.text else TvColors.textMuted,
            fontSize = 11.sp,
            fontWeight = if (selected) FontWeight.Bold else FontWeight.Medium,
            maxLines = 1
        )
    }
}

@Composable
private fun HomeSection(
    api: BuddyApi,
    serverName: String,
    hero: MediaEntry?,
    continueItems: List<MediaEntry>,
    recent: List<MediaEntry>,
    onPlay: (MediaEntry) -> Unit,
    onInfo: (MediaEntry) -> Unit
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(bottom = 40.dp)
    ) {
        hero?.let {
            item {
                Spotlight(
                    api = api,
                    serverName = serverName,
                    media = it,
                    onPlay = onPlay,
                    onInfo = onInfo
                )
            }
        }

        if (continueItems.isNotEmpty()) {
            item {
                Spacer(Modifier.height(12.dp))
                SectionTitle("Weiterschauen", "DEIN FORTSCHRITT")
                Spacer(Modifier.height(12.dp))
                LazyRow(
                    contentPadding = PaddingValues(horizontal = 34.dp),
                    horizontalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    items(continueItems.take(10), key = { "continue-" + it.id }) { media ->
                        LandscapeCard(api, media, onClick = { onPlay(media) })
                    }
                }
            }
        }

        if (recent.isNotEmpty()) {
            item {
                Spacer(Modifier.height(28.dp))
                SectionTitle("Neu bei BuddyFlix", "ZULETZT HINZUGEFÜGT")
                Spacer(Modifier.height(12.dp))
                LazyRow(
                    contentPadding = PaddingValues(horizontal = 34.dp),
                    horizontalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    items(recent, key = { "recent-" + it.id }) { media ->
                        PosterCard(api, media, onClick = { onPlay(media) })
                    }
                }
            }
        }
    }
}

@Composable
private fun Spotlight(
    api: BuddyApi,
    serverName: String,
    media: MediaEntry,
    onPlay: (MediaEntry) -> Unit,
    onInfo: (MediaEntry) -> Unit
) {
    val backdrop = api.imageUrl(media.backdrop.ifBlank { media.poster })

    Box(
        Modifier.fillMaxWidth().height(315.dp).background(TvColors.surfaceSoft)
    ) {
        if (backdrop.isNotBlank()) {
            AsyncImage(
                model = backdrop,
                contentDescription = null,
                modifier = Modifier.fillMaxSize(),
                contentScale = ContentScale.Crop
            )
        }

        Box(
            Modifier.fillMaxSize().background(
                Brush.horizontalGradient(
                    0f to Color(0xFD08090B),
                    .45f to Color(0xE808090B),
                    .78f to Color(0x7008090B),
                    1f to Color(0xB808090B)
                )
            )
        )

        Box(
            Modifier.fillMaxSize().background(
                Brush.verticalGradient(
                    0f to Color.Transparent,
                    .72f to Color(0x3308090B),
                    1f to TvColors.background
                )
            )
        )

        Column(
            Modifier.align(Alignment.BottomStart)
                .width(650.dp)
                .padding(start = 36.dp, bottom = 32.dp)
        ) {
            Text(
                "BUDDYFLIX · " + serverName.uppercase(),
                color = TvColors.textMuted,
                fontSize = 9.sp,
                fontWeight = FontWeight.Bold,
                letterSpacing = 2.sp
            )
            Spacer(Modifier.height(7.dp))
            Text(
                media.title,
                color = TvColors.text,
                fontSize = 42.sp,
                lineHeight = 44.sp,
                fontWeight = FontWeight.Bold,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis
            )
            Spacer(Modifier.height(6.dp))
            Text(media.subtitle, color = TvColors.textMuted, fontSize = 12.sp)
            if (media.overview.isNotBlank()) {
                Spacer(Modifier.height(9.dp))
                Text(
                    media.overview,
                    color = Color(0xFFD0D3D7),
                    fontSize = 12.sp,
                    lineHeight = 18.sp,
                    maxLines = 3,
                    overflow = TextOverflow.Ellipsis
                )
            }
            Spacer(Modifier.height(14.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(9.dp)) {
                ActionButton(
                    text = if (media.progress in 1.0..94.999) "▶  Fortsetzen" else "▶  Abspielen",
                    onClick = { onPlay(media) }
                )
                ActionButton(
                    text = "Details",
                    secondary = true,
                    onClick = { onInfo(media) }
                )
            }
        }
    }
}

@Composable
private fun MovieGridSection(
    api: BuddyApi,
    title: String,
    subtitle: String,
    movies: List<MediaEntry>,
    onPlay: (MediaEntry) -> Unit
) {
    Column(Modifier.fillMaxSize().padding(top = 30.dp)) {
        PageHeading(title, subtitle)

        if (movies.isEmpty()) {
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text("Hier ist noch nichts.", color = TvColors.textMuted, fontSize = 16.sp)
            }
        } else {
            LazyVerticalGrid(
                columns = GridCells.Fixed(5),
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(start = 34.dp, end = 34.dp, top = 22.dp, bottom = 34.dp),
                horizontalArrangement = Arrangement.spacedBy(14.dp),
                verticalArrangement = Arrangement.spacedBy(18.dp)
            ) {
                items(movies, key = { it.id }) { media ->
                    PosterCard(api, media, onClick = { onPlay(media) })
                }
            }
        }
    }
}

@Composable
private fun SeriesGridSection(
    api: BuddyApi,
    series: List<SeriesEntry>,
    onOpen: (SeriesEntry) -> Unit
) {
    Column(Modifier.fillMaxSize().padding(top = 30.dp)) {
        PageHeading("Serien", series.size.toString() + " Serien")

        if (series.isEmpty()) {
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text("Noch keine Serien gefunden.", color = TvColors.textMuted, fontSize = 16.sp)
            }
        } else {
            LazyVerticalGrid(
                columns = GridCells.Fixed(5),
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(start = 34.dp, end = 34.dp, top = 22.dp, bottom = 34.dp),
                horizontalArrangement = Arrangement.spacedBy(14.dp),
                verticalArrangement = Arrangement.spacedBy(18.dp)
            ) {
                items(series, key = { it.key }) { item ->
                    SeriesPosterCard(api, item, onClick = { onOpen(item) })
                }
            }
        }
    }
}

@Composable
private fun PageHeading(title: String, subtitle: String) {
    Column(Modifier.padding(horizontal = 34.dp)) {
        Text(title, color = TvColors.text, fontSize = 30.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(4.dp))
        Text(subtitle, color = TvColors.textMuted, fontSize = 11.sp)
    }
}

@Composable
private fun SectionTitle(title: String, kicker: String) {
    Column(Modifier.padding(horizontal = 34.dp)) {
        Text(kicker, color = TvColors.textDim, fontSize = 8.sp, fontWeight = FontWeight.Bold, letterSpacing = 2.sp)
        Spacer(Modifier.height(2.dp))
        Text(title, color = TvColors.text, fontSize = 22.sp, fontWeight = FontWeight.Bold)
    }
}

@Composable
private fun PosterCard(
    api: BuddyApi,
    media: MediaEntry,
    onClick: () -> Unit
) {
    var focused by remember { mutableStateOf(false) }
    val art = api.imageUrl(media.poster.ifBlank { media.backdrop })

    Column(
        Modifier.width(154.dp).height(282.dp)
            .onFocusChanged { focused = it.isFocused }
            .focusable()
            .clickable(onClick = onClick)
    ) {
        Box(
            Modifier.fillMaxWidth().height(228.dp)
                .clip(RoundedCornerShape(12.dp))
                .background(TvColors.surface)
                .border(
                    if (focused) 3.dp else 1.dp,
                    if (focused) TvColors.focus else TvColors.line,
                    RoundedCornerShape(12.dp)
                )
        ) {
            if (art.isNotBlank()) {
                AsyncImage(
                    model = art,
                    contentDescription = media.title,
                    modifier = Modifier.fillMaxSize(),
                    contentScale = ContentScale.Crop
                )
            }

            if (media.progress in 1.0..94.999) {
                Box(
                    Modifier.fillMaxWidth().height(4.dp)
                        .align(Alignment.BottomCenter)
                        .background(Color(0x663A3E44))
                ) {
                    Box(
                        Modifier.fillMaxWidth((media.progress / 100.0).toFloat().coerceIn(0f, 1f))
                            .fillMaxHeight()
                            .background(TvColors.progress)
                    )
                }
            }
        }

        Spacer(Modifier.height(8.dp))
        Text(
            media.title,
            color = if (focused) TvColors.text else Color(0xFFD7D9DC),
            fontSize = 12.sp,
            fontWeight = if (focused) FontWeight.Bold else FontWeight.Medium,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis
        )
        Spacer(Modifier.height(2.dp))
        Text(media.subtitle, color = TvColors.textDim, fontSize = 9.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

@Composable
private fun LandscapeCard(
    api: BuddyApi,
    media: MediaEntry,
    onClick: () -> Unit
) {
    var focused by remember { mutableStateOf(false) }
    val art = api.imageUrl(media.backdrop.ifBlank { media.poster })

    Column(
        Modifier.width(254.dp).height(184.dp)
            .onFocusChanged { focused = it.isFocused }
            .focusable()
            .clickable(onClick = onClick)
    ) {
        Box(
            Modifier.fillMaxWidth().height(143.dp)
                .clip(RoundedCornerShape(12.dp))
                .background(TvColors.surface)
                .border(
                    if (focused) 3.dp else 1.dp,
                    if (focused) TvColors.focus else TvColors.line,
                    RoundedCornerShape(12.dp)
                )
        ) {
            if (art.isNotBlank()) {
                AsyncImage(
                    model = art,
                    contentDescription = media.title,
                    modifier = Modifier.fillMaxSize(),
                    contentScale = ContentScale.Crop
                )
            }
            if (media.progress in 1.0..94.999) {
                Box(
                    Modifier.fillMaxWidth().height(4.dp)
                        .align(Alignment.BottomCenter)
                        .background(Color(0x663A3E44))
                ) {
                    Box(
                        Modifier.fillMaxWidth((media.progress / 100.0).toFloat().coerceIn(0f, 1f))
                            .fillMaxHeight()
                            .background(TvColors.progress)
                    )
                }
            }
        }

        Spacer(Modifier.height(8.dp))
        Text(
            media.title,
            color = if (focused) TvColors.text else Color(0xFFD7D9DC),
            fontSize = 12.sp,
            fontWeight = if (focused) FontWeight.Bold else FontWeight.Medium,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis
        )
    }
}

@Composable
private fun SeriesPosterCard(
    api: BuddyApi,
    series: SeriesEntry,
    onClick: () -> Unit
) {
    var focused by remember { mutableStateOf(false) }
    val art = api.imageUrl(series.poster.ifBlank { series.backdrop })

    Column(
        Modifier.width(154.dp).height(282.dp)
            .onFocusChanged { focused = it.isFocused }
            .focusable()
            .clickable(onClick = onClick)
    ) {
        Box(
            Modifier.fillMaxWidth().height(228.dp)
                .clip(RoundedCornerShape(12.dp))
                .background(TvColors.surface)
                .border(
                    if (focused) 3.dp else 1.dp,
                    if (focused) TvColors.focus else TvColors.line,
                    RoundedCornerShape(12.dp)
                )
        ) {
            if (art.isNotBlank()) {
                AsyncImage(
                    model = art,
                    contentDescription = series.title,
                    modifier = Modifier.fillMaxSize(),
                    contentScale = ContentScale.Crop
                )
            }
            Text(
                series.watchedCount.toString() + "/" + series.episodeCount,
                modifier = Modifier.align(Alignment.BottomEnd).padding(8.dp)
                    .background(Color(0xD008090B), RoundedCornerShape(20.dp))
                    .padding(horizontal = 7.dp, vertical = 3.dp),
                color = TvColors.text,
                fontSize = 8.sp,
                fontWeight = FontWeight.Bold
            )
        }

        Spacer(Modifier.height(8.dp))
        Text(
            series.title,
            color = if (focused) TvColors.text else Color(0xFFD7D9DC),
            fontSize = 12.sp,
            fontWeight = if (focused) FontWeight.Bold else FontWeight.Medium,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis
        )
        Spacer(Modifier.height(2.dp))
        Text(
            series.seasonCount.toString() + " Staffeln",
            color = TvColors.textDim,
            fontSize = 9.sp
        )
    }
}

@Composable
fun MovieInfoScreen(
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

    Box(Modifier.fillMaxSize().background(TvColors.background)) {
        if (backdrop.isNotBlank()) {
            AsyncImage(
                model = backdrop,
                contentDescription = null,
                modifier = Modifier.fillMaxSize(),
                contentScale = ContentScale.Crop
            )
        }

        Box(
            Modifier.fillMaxSize().background(
                Brush.horizontalGradient(
                    0f to Color(0xFF08090B),
                    .58f to Color(0xF208090B),
                    1f to Color(0xB808090B)
                )
            )
        )
        Box(
            Modifier.fillMaxSize().background(
                Brush.verticalGradient(
                    listOf(Color(0x2208090B), Color(0x8808090B), TvColors.background)
                )
            )
        )

        Row(
            Modifier.fillMaxSize().padding(horizontal = 54.dp, vertical = 44.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Box(
                Modifier.width(220.dp).aspectRatio(2f / 3f)
                    .clip(RoundedCornerShape(15.dp))
                    .background(TvColors.surface)
                    .border(1.dp, TvColors.line, RoundedCornerShape(15.dp))
            ) {
                if (poster.isNotBlank()) {
                    AsyncImage(
                        model = poster,
                        contentDescription = media.title,
                        modifier = Modifier.fillMaxSize(),
                        contentScale = ContentScale.Crop
                    )
                }
            }

            Spacer(Modifier.width(44.dp))

            Column(Modifier.width(620.dp)) {
                Text("FILM", color = TvColors.textDim, fontSize = 9.sp, fontWeight = FontWeight.Bold, letterSpacing = 2.sp)
                Spacer(Modifier.height(7.dp))
                Text(media.title, color = TvColors.text, fontSize = 44.sp, lineHeight = 46.sp, fontWeight = FontWeight.Bold)
                Spacer(Modifier.height(8.dp))
                Text(media.subtitle, color = TvColors.textMuted, fontSize = 12.sp)

                if (media.overview.isNotBlank()) {
                    Spacer(Modifier.height(15.dp))
                    Text(
                        media.overview,
                        color = Color(0xFFD0D3D7),
                        fontSize = 13.sp,
                        lineHeight = 20.sp,
                        maxLines = 6,
                        overflow = TextOverflow.Ellipsis
                    )
                }

                Spacer(Modifier.height(22.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(9.dp)) {
                    ActionButton(
                        text = if (media.progress in 1.0..94.999) "▶  Fortsetzen" else "▶  Abspielen",
                        onClick = { onPlay(media) }
                    )

                    ActionButton(
                        text = if (favorite) "✓  Meine Liste" else "+  Meine Liste",
                        secondary = true,
                        onClick = {
                            scope.launch {
                                runCatching {
                                    api.mediaAction(media.id, if (favorite) "unfavorite" else "favorite")
                                }.onSuccess {
                                    favorite = !favorite
                                    onChanged()
                                }
                            }
                        }
                    )

                    ActionButton(text = "Zurück", secondary = true, onClick = onBack)
                }
            }
        }
    }
}

@Composable
fun SeriesInfoScreen(
    api: BuddyApi,
    series: SeriesEntry,
    onPlay: (MediaEntry) -> Unit,
    onBack: () -> Unit
) {
    BackHandler(onBack = onBack)

    var selectedSeason by remember {
        mutableStateOf(series.nextEpisode?.season ?: series.seasons.firstOrNull()?.number ?: 0)
    }

    val season = series.seasons.firstOrNull { it.number == selectedSeason }
        ?: series.seasons.firstOrNull()
    val backdrop = api.imageUrl(series.backdrop.ifBlank { series.poster })

    Box(Modifier.fillMaxSize().background(TvColors.background)) {
        if (backdrop.isNotBlank()) {
            AsyncImage(
                model = backdrop,
                contentDescription = null,
                modifier = Modifier.fillMaxSize(),
                contentScale = ContentScale.Crop
            )
        }

        Box(
            Modifier.fillMaxSize().background(
                Brush.horizontalGradient(
                    0f to Color(0xFF08090B),
                    .62f to Color(0xE808090B),
                    1f to Color(0xB008090B)
                )
            )
        )
        Box(
            Modifier.fillMaxSize().background(
                Brush.verticalGradient(
                    listOf(Color(0x2208090B), Color(0xAA08090B), TvColors.background)
                )
            )
        )

        LazyColumn(
            Modifier.fillMaxSize(),
            contentPadding = PaddingValues(bottom = 42.dp)
        ) {
            item {
                Column(
                    Modifier.fillMaxWidth().height(285.dp)
                        .padding(horizontal = 48.dp, vertical = 34.dp),
                    verticalArrangement = Arrangement.Bottom
                ) {
                    Text("SERIE", color = TvColors.textDim, fontSize = 9.sp, fontWeight = FontWeight.Bold, letterSpacing = 2.sp)
                    Spacer(Modifier.height(7.dp))
                    Text(series.title, color = TvColors.text, fontSize = 46.sp, fontWeight = FontWeight.Bold)
                    Spacer(Modifier.height(7.dp))
                    Text(
                        series.seasonCount.toString() + " Staffeln · " +
                            series.episodeCount + " Folgen · " +
                            series.watchedCount + " gesehen",
                        color = TvColors.textMuted,
                        fontSize = 12.sp
                    )
                    Spacer(Modifier.height(14.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(9.dp)) {
                        series.nextEpisode?.let {
                            ActionButton(
                                text = if (it.progress in 1.0..94.999) "▶  Fortsetzen" else "▶  Nächste Folge",
                                onClick = { onPlay(it) }
                            )
                        }
                        ActionButton(text = "Zurück", secondary = true, onClick = onBack)
                    }
                }
            }

            item {
                SectionTitle("Staffeln", "AUSWÄHLEN")
                Spacer(Modifier.height(10.dp))
                LazyRow(
                    contentPadding = PaddingValues(horizontal = 34.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    items(series.seasons, key = { it.number }) { item ->
                        ActionButton(
                            text = if (item.number == 0) "Extras" else "Staffel " + item.number,
                            secondary = item.number != selectedSeason,
                            compact = true,
                            onClick = { selectedSeason = item.number }
                        )
                    }
                }
            }

            item {
                Spacer(Modifier.height(24.dp))
                SectionTitle(
                    if ((season?.number ?: 0) == 0) "Extras" else "Staffel " + season?.number,
                    (season?.episodes?.size ?: 0).toString() + " FOLGEN"
                )
                Spacer(Modifier.height(12.dp))
                LazyRow(
                    contentPadding = PaddingValues(horizontal = 34.dp),
                    horizontalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    items(season?.episodes.orEmpty(), key = { it.id }) { episode ->
                        LandscapeCard(api, episode, onClick = { onPlay(episode) })
                    }
                }
            }
        }
    }
}

@Composable
private fun FocusSurface(
    modifier: Modifier = Modifier,
    onClick: () -> Unit,
    content: @Composable ColumnScope.() -> Unit
) {
    var focused by remember { mutableStateOf(false) }

    Column(
        modifier = modifier
            .onFocusChanged { focused = it.isFocused }
            .background(
                if (focused) TvColors.surfaceRaised else TvColors.surface,
                RoundedCornerShape(14.dp)
            )
            .border(
                if (focused) 2.dp else 1.dp,
                if (focused) TvColors.focus else TvColors.line,
                RoundedCornerShape(14.dp)
            )
            .focusable()
            .clickable(onClick = onClick),
        content = content
    )
}

@Composable
private fun ActionButton(
    text: String,
    secondary: Boolean = false,
    compact: Boolean = false,
    enabled: Boolean = true,
    onClick: () -> Unit
) {
    var focused by remember { mutableStateOf(false) }

    val bg = when {
        !enabled -> Color(0xFF202226)
        focused -> TvColors.text
        secondary -> Color(0xB315171B)
        else -> Color(0xFFE9EBEE)
    }

    val fg = when {
        !enabled -> TvColors.textDim
        focused -> TvColors.background
        secondary -> TvColors.text
        else -> TvColors.background
    }

    Box(
        Modifier
            .height(if (compact) 38.dp else 44.dp)
            .background(bg, RoundedCornerShape(10.dp))
            .border(
                if (focused) 2.dp else 1.dp,
                if (focused) TvColors.focus else if (secondary) TvColors.line else Color.Transparent,
                RoundedCornerShape(10.dp)
            )
            .onFocusChanged { focused = it.isFocused }
            .focusable(enabled)
            .clickable(enabled = enabled, onClick = onClick)
            .padding(horizontal = if (compact) 14.dp else 18.dp),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text,
            color = fg,
            fontSize = if (compact) 10.sp else 11.sp,
            fontWeight = FontWeight.Bold,
            maxLines = 1
        )
    }
}
