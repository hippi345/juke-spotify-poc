package com.juke.spotifypoc.mobile.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import coil.compose.AsyncImage
import com.juke.spotifypoc.mobile.R
import com.juke.spotifypoc.mobile.VenueViewModel
import com.juke.spotifypoc.mobile.api.NearbySession
import com.juke.spotifypoc.mobile.api.NowPlayingBlock
import com.juke.spotifypoc.mobile.api.Track
import com.juke.spotifypoc.mobile.api.TrackWithMeta

private val ScreenGradient = Brush.verticalGradient(
    colors = listOf(
        Color(0xFF050506),
        Color(0xFF0C0C0E),
        Color(0xFF12121A),
    ),
)

@Composable
fun VenueApp(
    vm: VenueViewModel = viewModel(),
    onRequestLocationPermission: () -> Unit = {},
) {
    val ui by vm.ui.collectAsState()

    LaunchedEffect(ui.hasJoinedSession) {
        if (ui.hasJoinedSession) {
            vm.startPolling()
        }
    }

    val uriHandler = LocalUriHandler.current
    LaunchedEffect(ui.paidSkipCheckoutUrl) {
        val url = ui.paidSkipCheckoutUrl
        if (!url.isNullOrBlank()) {
            uriHandler.openUri(url)
            vm.clearPaidSkipCheckoutUrl()
        }
    }

    val initialLoading = ui.loading && ui.patronEmail == null && ui.pollError == null

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(ScreenGradient)
            .statusBarsPadding()
            .navigationBarsPadding(),
    ) {
        if (initialLoading) {
            BrandedLaunchLoading(Modifier.fillMaxSize())
        } else {
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .verticalScroll(rememberScrollState())
                    .padding(horizontal = 12.dp, vertical = 8.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
            when {
                ui.patronEmail == null -> {
                    PatronAuthSection(
                        authError = ui.authError,
                        authLoading = ui.authLoading,
                        onLogin = vm::login,
                        onRegister = vm::register,
                    )
                }
                !ui.hasJoinedSession -> {
                    PatronDiscoverSection(
                        ui = ui,
                        onRequestLocationPermission = onRequestLocationPermission,
                        onUseLocation = vm::useDeviceLocation,
                        onLoadNearby = vm::loadNearby,
                        onJoin = vm::prepareJoin,
                        onConfirmJoin = vm::confirmJoin,
                        onCancelJoin = vm::cancelJoin,
                        onJoinPasswordChange = vm::setJoinPassword,
                        onLatChange = vm::setManualLat,
                        onLngChange = vm::setManualLng,
                        onLogout = vm::logout,
                    )
                }
                else -> {
            ui.pollError?.let { err ->
                GlassPanel(Modifier.fillMaxWidth()) {
                    Text(
                        "Cannot load voting state: $err",
                        modifier = Modifier.padding(12.dp),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.error,
                    )
                }
            }

            val session = ui.votingState?.session
            val sessionActive = session != null && session.status == "active"
            when {
                !sessionActive -> {
                    GlassPanel(Modifier.fillMaxWidth()) {
                        Text(
                            "Joined ${ui.joinedVenueName ?: "venue"} — waiting for host to start playback.",
                            modifier = Modifier.padding(16.dp),
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.85f),
                        )
                    }
                }
                else -> {
                    NowPlayingSection(nowPlaying = ui.votingState?.nowPlaying)
                    VotingSection(
                        candidates = ui.votingState?.candidates.orEmpty(),
                        votes = ui.votingState?.votes,
                        timeRemainingSec = ui.votingState?.timeRemainingSec ?: 0,
                        voteError = ui.voteError,
                        votingTrackId = ui.votingTrackId,
                        hasVotedThisRound = ui.hasVotedThisRound,
                        onVote = { vm.vote(it) },
                    )
                    ui.paidSkipError?.let { err ->
                        Text(
                            err,
                            color = MaterialTheme.colorScheme.error,
                            style = MaterialTheme.typography.bodySmall,
                            modifier = Modifier.padding(horizontal = 4.dp),
                        )
                    }
                    PlaylistSection(
                        tracks = ui.playlistTracks,
                        error = ui.playlistError,
                        paidSkipEnabled = ui.paidSkipEnabled,
                        paidSkipPriceUsd = ui.paidSkipPriceUsd,
                        paidSkipLoadingTrackId = ui.paidSkipTrackId,
                        onPaidSkip = { vm.startPaidSkip(it) },
                    )
                }
            }
                }
            }
            Spacer(modifier = Modifier.height(24.dp))
            }
        }
    }
}

@Composable
private fun PatronAuthSection(
    authError: String?,
    authLoading: Boolean,
    onLogin: (String, String) -> Unit,
    onRegister: (String, String) -> Unit,
) {
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    GlassPanel(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Text("Patron sign in", style = MaterialTheme.typography.titleMedium, color = Color.White)
            OutlinedTextField(
                value = email,
                onValueChange = { email = it },
                label = { Text("Email") },
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedTextField(
                value = password,
                onValueChange = { password = it },
                label = { Text("Password") },
                modifier = Modifier.fillMaxWidth(),
            )
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(
                    onClick = { onLogin(email, password) },
                    enabled = !authLoading,
                ) { Text("Sign in") }
                Button(
                    onClick = { onRegister(email, password) },
                    enabled = !authLoading,
                ) { Text("Register") }
            }
            authError?.let {
                Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
            }
        }
    }
}

@Composable
private fun PatronDiscoverSection(
    ui: com.juke.spotifypoc.mobile.VenueUiState,
    onRequestLocationPermission: () -> Unit,
    onUseLocation: () -> Unit,
    onLoadNearby: () -> Unit,
    onJoin: (Long) -> Unit,
    onConfirmJoin: () -> Unit,
    onCancelJoin: () -> Unit,
    onJoinPasswordChange: (String) -> Unit,
    onLatChange: (String) -> Unit,
    onLngChange: (String) -> Unit,
    onLogout: () -> Unit,
) {
    GlassPanel(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                Text("Find a session", style = MaterialTheme.typography.titleMedium, color = Color.White)
                Text(
                    "Sign out",
                    modifier = Modifier.clickable { onLogout() },
                    color = MaterialTheme.colorScheme.primary,
                    style = MaterialTheme.typography.labelMedium,
                )
            }
            OutlinedTextField(
                value = ui.manualLat,
                onValueChange = onLatChange,
                label = { Text("Latitude") },
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedTextField(
                value = ui.manualLng,
                onValueChange = onLngChange,
                label = { Text("Longitude") },
                modifier = Modifier.fillMaxWidth(),
            )
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(onClick = {
                    onRequestLocationPermission()
                    onUseLocation()
                }) { Text("Use location") }
                Button(onClick = onLoadNearby, enabled = !ui.discoverLoading) {
                    Text(if (ui.discoverLoading) "Loading…" else "Search nearby")
                }
            }
            ui.discoverError?.let {
                Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
            }
            ui.nearbySessions.forEach { row -> NearbyRow(row, onJoin) }
            if (ui.joinVenueId != null) {
                val needsPwd = ui.nearbySessions.firstOrNull { it.venue?.id == ui.joinVenueId }?.requiresPassword == true
                if (needsPwd) {
                    OutlinedTextField(
                        value = ui.joinPassword,
                        onValueChange = onJoinPasswordChange,
                        label = { Text("Join password") },
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = onConfirmJoin) { Text("Join session") }
                    Button(onClick = onCancelJoin) { Text("Cancel") }
                }
            }
        }
    }
}

@Composable
private fun NearbyRow(row: NearbySession, onJoin: (Long) -> Unit) {
    val venue = row.venue
    if (venue == null) return
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { onJoin(venue.id) },
        colors = CardDefaults.cardColors(containerColor = Color.White.copy(alpha = 0.06f)),
    ) {
        Column(Modifier.padding(12.dp)) {
            Text(venue.name, fontWeight = FontWeight.SemiBold, color = Color.White)
            Text(
                "${row.distanceM} m · ${row.playlistName ?: "Playlist"}${if (row.requiresPassword) " · password" else ""}",
                style = MaterialTheme.typography.bodySmall,
                color = Color.White.copy(alpha = 0.75f),
            )
        }
    }
}

@Composable
private fun BrandedLaunchLoading(modifier: Modifier = Modifier) {
    Box(
        modifier = modifier
            .fillMaxSize()
            .padding(horizontal = 12.dp, vertical = 8.dp),
        contentAlignment = Alignment.Center,
    ) {
        GlassPanel(Modifier.fillMaxWidth()) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 24.dp, vertical = 28.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                Image(
                    painter = painterResource(R.drawable.juke_logo),
                    contentDescription = "juke",
                    modifier = Modifier.size(128.dp),
                )
                CircularProgressIndicator(
                    modifier = Modifier.padding(top = 8.dp),
                    color = MaterialTheme.colorScheme.primary,
                )
            }
        }
    }
}

@Composable
private fun GlassPanel(
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit,
) {
    Card(
        modifier = modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        border = BorderStroke(1.dp, Color.White.copy(alpha = 0.1f)),
        colors = CardDefaults.cardColors(
            containerColor = Color.White.copy(alpha = 0.06f),
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 0.dp),
    ) {
        content()
    }
}

@Composable
private fun NowPlayingSection(nowPlaying: NowPlayingBlock?) {
    val item = nowPlaying?.item
    val uriHandler = LocalUriHandler.current
    GlassPanel(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp)) {
            Text(
                "Now playing",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
                color = Color.White.copy(alpha = 0.95f),
            )
            Spacer(Modifier.height(12.dp))
            if (item == null) {
                Text(
                    "Nothing playing",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                )
            } else {
                Row(
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    val img = item.album?.images?.firstOrNull()?.url
                    if (!img.isNullOrBlank()) {
                        AsyncImage(
                            model = img,
                            contentDescription = item.album?.name,
                            modifier = Modifier
                                .height(100.dp)
                                .aspectRatio(1f)
                                .clip(RoundedCornerShape(8.dp)),
                            contentScale = ContentScale.Crop,
                        )
                    }
                    Column(Modifier.weight(1f)) {
                        Text(
                            item.name,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                            style = MaterialTheme.typography.titleSmall,
                            fontWeight = FontWeight.Medium,
                            color = Color.White.copy(alpha = 0.95f),
                        )
                        Text(
                            item.artists?.joinToString { it.name }.orEmpty(),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.65f),
                        )
                        Row(
                            modifier = Modifier.padding(top = 4.dp),
                            horizontalArrangement = Arrangement.spacedBy(10.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            if (nowPlaying?.playing == true) {
                                Text(
                                    "Playing",
                                    style = MaterialTheme.typography.labelSmall,
                                    color = MaterialTheme.colorScheme.primary,
                                )
                            }
                            if (item.id.isNotBlank()) {
                                val spotifyUrl = "https://open.spotify.com/track/${item.id}"
                                Text(
                                    "Open in Spotify",
                                    style = MaterialTheme.typography.labelSmall,
                                    color = Color(0xFF1DB954),
                                    modifier = Modifier.clickable {
                                        uriHandler.openUri(spotifyUrl)
                                    },
                                )
                            }
                        }
                    }
                }
                if (item.durationMs > 0) {
                    val progress = (nowPlaying?.progressMs ?: 0L).coerceIn(0, item.durationMs)
                    val pct = (progress.toFloat() / item.durationMs.toFloat()).coerceIn(0f, 1f)
                    Spacer(Modifier.height(12.dp))
                    Row(
                        Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                    ) {
                        Text(
                            formatMs(progress),
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f),
                        )
                        Text(
                            formatMs((item.durationMs - progress).coerceAtLeast(0)),
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f),
                        )
                    }
                    LinearProgressIndicator(
                        progress = { pct },
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(4.dp)
                            .clip(RoundedCornerShape(2.dp)),
                    )
                }
            }
        }
    }
}

@Composable
private fun VotingSection(
    candidates: List<Track>,
    votes: Map<String, Double>?,
    timeRemainingSec: Int,
    voteError: String?,
    votingTrackId: String?,
    hasVotedThisRound: Boolean,
    onVote: (String) -> Unit,
) {
    val votingEnded = timeRemainingSec <= 0
    val canVote = !votingEnded && votingTrackId == null && !hasVotedThisRound

    GlassPanel(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp)) {
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.End,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(
                    modifier = Modifier
                        .clip(RoundedCornerShape(999.dp))
                        .background(
                            if (votingEnded) Color.White.copy(alpha = 0.08f)
                            else Color(0xFF1DB954).copy(alpha = 0.15f),
                        )
                        .padding(horizontal = 10.dp, vertical = 6.dp),
                ) {
                    Text(
                        if (votingEnded) "Voting ended" else "${timeRemainingSec}s",
                        style = MaterialTheme.typography.labelLarge,
                        color = if (votingEnded) {
                            MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f)
                        } else {
                            MaterialTheme.colorScheme.primary
                        },
                    )
                }
            }
            if (hasVotedThisRound && !votingEnded) {
                Spacer(Modifier.height(8.dp))
                Text(
                    "You voted this round",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.primary,
                )
            }
            voteError?.let {
                Spacer(Modifier.height(8.dp))
                Text(
                    it,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.error,
                )
            }
            Spacer(Modifier.height(12.dp))
            if (candidates.isEmpty()) {
                Text(
                    "Loading next round…",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                )
            } else {
                Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    candidates.forEach { track ->
                        VoteCandidateRow(
                            track = track,
                            voteCount = votes?.get(track.id)?.toInt() ?: 0,
                            enabled = canVote,
                            busy = votingTrackId == track.id,
                            onVote = { onVote(track.id) },
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun VoteCandidateRow(
    track: Track,
    voteCount: Int,
    enabled: Boolean,
    busy: Boolean,
    onVote: () -> Unit,
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        val img = track.album?.images?.firstOrNull()?.url
        if (!img.isNullOrBlank()) {
            AsyncImage(
                model = img,
                contentDescription = null,
                modifier = Modifier
                    .height(56.dp)
                    .aspectRatio(1f)
                    .clip(RoundedCornerShape(6.dp)),
                contentScale = ContentScale.Crop,
            )
        }
        val trackMetaColor = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.65f)
        Column(Modifier.weight(1f)) {
            Text(
                track.name,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
                style = MaterialTheme.typography.bodyMedium,
                color = trackMetaColor,
            )
            Text(
                track.artists?.joinToString { it.name }.orEmpty(),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                style = MaterialTheme.typography.labelSmall,
                color = trackMetaColor,
            )
        }
        Text(
            "$voteCount",
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.7f),
            modifier = Modifier.padding(end = 4.dp),
        )
        Button(
            onClick = onVote,
            enabled = enabled && !busy,
            colors = ButtonDefaults.buttonColors(
                containerColor = MaterialTheme.colorScheme.primary,
            ),
        ) {
            if (busy) {
                CircularProgressIndicator(
                    strokeWidth = 2.dp,
                    modifier = Modifier.size(18.dp),
                    color = MaterialTheme.colorScheme.onPrimary,
                )
            } else {
                Text("Vote")
            }
        }
    }
}

@Composable
private fun PlaylistSection(
    tracks: List<TrackWithMeta>,
    error: String?,
    paidSkipEnabled: Boolean,
    paidSkipPriceUsd: String,
    paidSkipLoadingTrackId: String?,
    onPaidSkip: (String) -> Unit,
) {
    GlassPanel(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp)) {
            Text(
                "Playlist",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
                color = Color.White.copy(alpha = 0.95f),
            )
            if (paidSkipEnabled) {
                Text(
                    "Pay $paidSkipPriceUsd (Stripe test) to play a track next — must be on this playlist.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.65f),
                    modifier = Modifier.padding(top = 4.dp),
                )
            }
            Spacer(Modifier.height(8.dp))
            error?.let {
                Text(
                    it,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.error,
                )
                Spacer(Modifier.height(8.dp))
            }
            if (tracks.isEmpty()) {
                Text(
                    "No tracks loaded yet.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f),
                )
            } else {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    tracks.chunked(3).forEach { rowTracks ->
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            rowTracks.forEach { cell ->
                                Box(Modifier.weight(1f)) {
                                    PlaylistTile(
                                        cell,
                                        paidSkipEnabled = paidSkipEnabled && !cell.played,
                                        paidSkipLoading = paidSkipLoadingTrackId == cell.track.id,
                                        paidSkipPriceUsd = paidSkipPriceUsd,
                                        onPaidSkip = { onPaidSkip(cell.track.id) },
                                    )
                                }
                            }
                            repeat(3 - rowTracks.size) {
                                Spacer(Modifier.weight(1f))
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun PlaylistTile(
    row: TrackWithMeta,
    paidSkipEnabled: Boolean,
    paidSkipLoading: Boolean,
    paidSkipPriceUsd: String,
    onPaidSkip: () -> Unit,
) {
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box {
            val img = row.track.album?.images?.firstOrNull()?.url
            if (!img.isNullOrBlank()) {
                AsyncImage(
                    model = img,
                    contentDescription = row.track.name,
                    modifier = Modifier
                        .fillMaxWidth()
                        .aspectRatio(1f)
                        .clip(RoundedCornerShape(8.dp)),
                    contentScale = ContentScale.Crop,
                )
            } else {
                Box(
                    Modifier
                        .fillMaxWidth()
                        .aspectRatio(1f)
                        .background(MaterialTheme.colorScheme.onSurface.copy(alpha = 0.12f), RoundedCornerShape(8.dp)),
                )
            }
            if (row.played) {
                Box(
                    Modifier
                        .fillMaxSize()
                        .background(Color.Black.copy(alpha = 0.45f), RoundedCornerShape(8.dp)),
                    contentAlignment = Alignment.Center,
                ) {
                    Text("▶", color = MaterialTheme.colorScheme.primary, fontWeight = FontWeight.Bold)
                }
            }
            if (row.refilled && !row.played) {
                Box(
                    Modifier
                        .fillMaxSize()
                        .padding(4.dp),
                    contentAlignment = Alignment.BottomEnd,
                ) {
                    Text(
                        "=",
                        modifier = Modifier
                            .background(Color.Black.copy(alpha = 0.55f), RoundedCornerShape(4.dp))
                            .padding(horizontal = 4.dp, vertical = 2.dp),
                        color = MaterialTheme.colorScheme.primary,
                        style = MaterialTheme.typography.labelSmall,
                        fontWeight = FontWeight.Bold,
                    )
                }
            }
        }
        Text(
            row.track.name,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
            textAlign = TextAlign.Center,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.75f),
            modifier = Modifier.padding(top = 4.dp),
        )
        if (paidSkipEnabled) {
            Button(
                onClick = onPaidSkip,
                enabled = !paidSkipLoading,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 4.dp),
                contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 4.dp, vertical = 2.dp),
            ) {
                if (paidSkipLoading) {
                    CircularProgressIndicator(
                        strokeWidth = 2.dp,
                        modifier = Modifier.size(14.dp),
                    )
                } else {
                    Text("Skip $paidSkipPriceUsd", style = MaterialTheme.typography.labelSmall)
                }
            }
        }
    }
}
