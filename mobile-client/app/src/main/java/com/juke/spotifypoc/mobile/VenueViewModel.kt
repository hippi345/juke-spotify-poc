package com.juke.spotifypoc.mobile

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.juke.spotifypoc.mobile.api.ApiClient
import com.juke.spotifypoc.mobile.api.TrackWithMeta
import com.juke.spotifypoc.mobile.api.VoteRequest
import com.juke.spotifypoc.mobile.api.VotingApi
import com.juke.spotifypoc.mobile.api.VotingStateResponse
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

data class VenueUiState(
    val loading: Boolean = true,
    val pollError: String? = null,
    val votingState: VotingStateResponse? = null,
    /** Stable id for the current voting round (session + candidate set). Used only on mobile to cap one vote per round. */
    val roundKey: String = "",
    /** Mobile-only: after a successful vote, stay disabled until the round changes. */
    val hasVotedThisRound: Boolean = false,
    val playlistTracks: List<TrackWithMeta> = emptyList(),
    val playlistError: String? = null,
    val voteError: String? = null,
    val votingTrackId: String? = null,
)

private fun computeVotingRoundKey(s: VotingStateResponse): String {
    val sess = s.session
    if (sess == null || sess.status != "active") return ""
    val sessionPart = listOfNotNull(sess.id?.toString(), sess.playlistId).firstOrNull() ?: "active"
    val ids = s.candidates.orEmpty().map { it.id }.sorted().joinToString("|")
    return "$sessionPart|$ids"
}

class VenueViewModel : ViewModel() {

    private val api: VotingApi = ApiClient.create()

    private val _ui = MutableStateFlow(VenueUiState())
    val ui: StateFlow<VenueUiState> = _ui.asStateFlow()

    private var pollJob: Job? = null

    fun startPolling() {
        pollJob?.cancel()
        pollJob = viewModelScope.launch {
            while (isActive) {
                try {
                    val s = api.getState()
                    val rk = computeVotingRoundKey(s)
                    _ui.update { prev ->
                        val sessionActive = s.session != null && s.session.status == "active"
                        val roundChanged = rk != prev.roundKey
                        val newHasVoted = when {
                            !sessionActive -> false
                            roundChanged -> false
                            else -> prev.hasVotedThisRound
                        }
                        prev.copy(
                            votingState = s,
                            roundKey = rk,
                            hasVotedThisRound = newHasVoted,
                            pollError = null,
                            loading = false,
                        )
                    }
                    if (s.session != null && s.session.status == "active") {
                        try {
                            val pl = api.getPlaylistOverview()
                            _ui.update {
                                it.copy(
                                    playlistTracks = pl.tracks.orEmpty(),
                                    playlistError = null,
                                )
                            }
                        } catch (e: Exception) {
                            _ui.update { it.copy(playlistError = e.message ?: "playlist") }
                        }
                    } else {
                        _ui.update { it.copy(playlistTracks = emptyList(), playlistError = null) }
                    }
                } catch (e: Exception) {
                    _ui.update {
                        it.copy(
                            pollError = e.message ?: "Could not reach server",
                            loading = false,
                        )
                    }
                }
                delay(2000)
            }
        }
    }

    fun vote(trackId: String) {
        viewModelScope.launch {
            if (_ui.value.hasVotedThisRound) return@launch
            _ui.update { it.copy(votingTrackId = trackId, voteError = null) }
            try {
                api.vote(VoteRequest(trackId))
                _ui.update { it.copy(hasVotedThisRound = true, votingTrackId = null) }
            } catch (e: Exception) {
                _ui.update { it.copy(voteError = e.message ?: "Vote failed", votingTrackId = null) }
            }
        }
    }

    override fun onCleared() {
        pollJob?.cancel()
        super.onCleared()
    }
}
