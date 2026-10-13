package de.buddyflix.firetv

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.media3.common.MediaItem
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.ui.PlayerView
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

@Composable
fun PlayerScreen(
    api: BuddyApi,
    media: MediaEntry,
    onBack: () -> Unit
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var playbackError by remember { mutableStateOf<String?>(null) }

    val player = remember(media.id, api.baseUrl, api.token, api.profileId) {
        val httpFactory = DefaultHttpDataSource.Factory()
            .setConnectTimeoutMs(8_000)
            .setReadTimeoutMs(30_000)
            .setAllowCrossProtocolRedirects(true)
            .setDefaultRequestProperties(api.streamHeaders())

        val mediaSourceFactory = DefaultMediaSourceFactory(httpFactory)

        ExoPlayer.Builder(context)
            .setMediaSourceFactory(mediaSourceFactory)
            .build()
            .apply {
                addListener(object : Player.Listener {
                    override fun onPlayerError(error: PlaybackException) {
                        playbackError = error.message ?: "Wiedergabe fehlgeschlagen."
                    }
                })

                setMediaItem(MediaItem.fromUri(api.streamUrl(media.id)))

                if (media.position > 20.0 && media.progress < 95.0) {
                    seekTo((media.position * 1000.0).toLong())
                }

                prepare()
                playWhenReady = true
                play()
            }
    }

    fun saveProgress() {
        val durationMs = player.duration
        val positionMs = player.currentPosition
        if (durationMs > 0L && positionMs >= 0L) {
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
        saveProgress()
        onBack()
    }

    LaunchedEffect(player) {
        while (true) {
            delay(10_000)
            saveProgress()
        }
    }

    DisposableEffect(player) {
        onDispose {
            saveProgress()
            player.release()
        }
    }

    Box(Modifier.fillMaxSize().background(Color.Black)) {
        AndroidView(
            modifier = Modifier.fillMaxSize().onPreviewKeyEvent {
                if (it.key == Key.Back) {
                    saveProgress()
                    onBack()
                    true
                } else {
                    false
                }
            },
            factory = { ctx ->
                PlayerView(ctx).apply {
                    useController = true
                    controllerAutoShow = true
                    controllerShowTimeoutMs = 3500
                    this.player = player
                    requestFocus()
                }
            },
            update = { it.player = player }
        )

        playbackError?.let {
            Column(
                Modifier.align(Alignment.Center)
                    .background(Color(0xE608090B))
                    .padding(horizontal = 28.dp, vertical = 22.dp)
            ) {
                Text("Film konnte nicht gestartet werden.", color = TvColors.text, fontSize = 18.sp)
                Spacer(Modifier.height(8.dp))
                Text(it, color = TvColors.error, fontSize = 11.sp)
                Spacer(Modifier.height(8.dp))
                Text("Zurück drücken, um zur Bibliothek zu wechseln.", color = TvColors.textMuted, fontSize = 10.sp)
            }
        }
    }
}
