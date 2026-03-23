package com.juke.spotifypoc.mobile.ui

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

private val JukeGreen = Color(0xFF1DB954)
private val Bg = Color(0xFF0C0C0E)

private val ColorScheme = darkColorScheme(
    primary = JukeGreen,
    onPrimary = Color.White,
    secondary = Color(0xFF1ED760),
    background = Bg,
    surface = Color(0xFF141416),
    onSurface = Color(0xFFE4E4E7),
    onBackground = Color(0xFFE4E4E7),
)

@Composable
fun JukeVenueTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = ColorScheme,
        content = content,
    )
}
