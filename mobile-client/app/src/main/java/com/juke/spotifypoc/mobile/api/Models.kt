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
