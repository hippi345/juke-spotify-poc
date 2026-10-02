package com.juke.spotifypoc.mobile.api

import com.google.gson.annotations.SerializedName

data class Artist(
    val id: String = "",
    val name: String = "",
)

data class AlbumImage(
    val url: String = "",
    val width: Int = 0,
    val height: Int = 0,
)

data class Album(
    val id: String = "",
    val name: String = "",
    val images: List<AlbumImage>? = null,
)

data class Track(
    val id: String = "",
    val name: String = "",
    val uri: String? = null,
    val artists: List<Artist>? = null,
    val album: Album? = null,
    @SerializedName("duration_ms") val durationMs: Long = 0,
)

data class SessionInfo(
    val id: Long? = null,
    @SerializedName("playlist_id") val playlistId: String? = null,
    @SerializedName("playlist_name") val playlistName: String? = null,
    @SerializedName("refill_threshold") val refillThreshold: Int = 0,
    @SerializedName("refill_count") val refillCount: Int = 0,
    @SerializedName("keep_refill_tracks") val keepRefillTracks: Boolean = false,
    val status: String? = null,
)

data class NowPlayingBlock(
    val playing: Boolean = false,
    @SerializedName("progress_ms") val progressMs: Long = 0,
    val item: Track? = null,
)

data class VotingStateResponse(
    val session: SessionInfo? = null,
    @SerializedName("now_playing") val nowPlaying: NowPlayingBlock? = null,
    val candidates: List<Track>? = null,
    /** Vote counts keyed by track id */
    val votes: Map<String, Double>? = null,
    @SerializedName("time_remaining_sec") val timeRemainingSec: Int = 0,
    @SerializedName("playlist_updated_at") val playlistUpdatedAt: Double? = null,
)

data class VoteRequest(
    @SerializedName("track_id") val trackId: String,
)

data class TrackWithMeta(
    val track: Track,
    val played: Boolean = false,
    val refilled: Boolean = false,
)

data class PlaylistOverviewResponse(
    val tracks: List<TrackWithMeta>? = null,
)

data class RegisterRequest(
    val email: String,
    val password: String,
    val role: String = "patron",
)

data class LoginRequest(
    val email: String,
    val password: String,
)

data class AuthUser(
    val id: Long = 0,
    val email: String = "",
    val role: String = "",
)

data class AuthResponse(
    val token: String = "",
    val user: AuthUser? = null,
)

data class VenueInfo(
    val id: Long = 0,
    val name: String = "",
    val latitude: Double = 0.0,
    val longitude: Double = 0.0,
)

data class NearbySession(
    val venue: VenueInfo? = null,
    @SerializedName("session_id") val sessionId: Long = 0,
    @SerializedName("playlist_name") val playlistName: String? = null,
    @SerializedName("distance_m") val distanceM: Int = 0,
    @SerializedName("requires_password") val requiresPassword: Boolean = false,
)

data class NearbyResponse(
    val sessions: List<NearbySession>? = null,
)

data class JoinRequest(
    @SerializedName("join_password") val joinPassword: String? = null,
)

data class JoinResponse(
    val status: String = "",
    val venue: VenueInfo? = null,
    @SerializedName("session_id") val sessionId: Long = 0,
)
