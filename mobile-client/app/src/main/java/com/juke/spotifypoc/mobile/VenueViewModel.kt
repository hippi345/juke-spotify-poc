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
    val playlistTracks: List<TrackWithMeta> = emptyList(),
    val playlistError: String? = null,
    val voteError: String? = null,
    val votingTrackId: String? = null,
)

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
                    _ui.update {
                        it.copy(
                            votingState = s,
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
            _ui.update { it.copy(votingTrackId = trackId, voteError = null) }
            try {
                api.vote(VoteRequest(trackId))
            } catch (e: Exception) {
                _ui.update { it.copy(voteError = e.message ?: "Vote failed") }
            } finally {
                _ui.update { it.copy(votingTrackId = null) }
            }
        }
    }

    override fun onCleared() {
        pollJob?.cancel()
        super.onCleared()
    }
}
