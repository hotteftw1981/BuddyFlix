package de.buddyflix.firetv

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

object TvColors {
    val background = Color(0xFF08090B)
    val surface = Color(0xFF111317)
    val surfaceRaised = Color(0xFF181B20)
    val surfaceSoft = Color(0xFF0D0F12)
    val text = Color(0xFFF5F6F7)
    val textMuted = Color(0xFF92979F)
    val textDim = Color(0xFF686E77)
    val line = Color(0xFF2A2E35)
    val focus = Color(0xFFF3F5F7)
    val brand = Color(0xFFFF6D4A)
    val progress = Color(0xFFE4E7EB)
    val error = Color(0xFFFF8F99)
}

private val BuddyTvColorScheme = darkColorScheme(
    primary = TvColors.text,
    onPrimary = TvColors.background,
    background = TvColors.background,
    onBackground = TvColors.text,
    surface = TvColors.surface,
    onSurface = TvColors.text,
    surfaceVariant = TvColors.surfaceRaised,
    onSurfaceVariant = TvColors.textMuted,
    outline = TvColors.line,
    error = TvColors.error
)

@Composable
fun BuddyFlixTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = BuddyTvColorScheme,
        content = content
    )
}
