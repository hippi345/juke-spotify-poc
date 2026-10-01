package com.juke.spotifypoc.mobile

import android.annotation.SuppressLint
import android.app.Application
import android.content.Context
import android.location.LocationManager
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.juke.spotifypoc.mobile.api.ApiClient
import com.juke.spotifypoc.mobile.api.JoinRequest
import com.juke.spotifypoc.mobile.api.LoginRequest
import com.juke.spotifypoc.mobile.api.NearbySession
import com.juke.spotifypoc.mobile.api.RegisterRequest
import com.juke.spotifypoc.mobile.api.TrackWithMeta
import com.juke.spotifypoc.mobile.api.VoteRequest
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
    val roundKey: String = "",
    val hasVotedThisRound: Boolean = false,
    val playlistTracks: List<TrackWithMeta> = emptyList(),
    val playlistError: String? = null,
    val voteError: String? = null,
    val votingTrackId: String? = null,
    val patronEmail: String? = null,
    val authError: String? = null,
    val authLoading: Boolean = false,
    val nearbySessions: List<NearbySession> = emptyList(),
    val discoverError: String? = null,
    val discoverLoading: Boolean = false,
    val joinedVenueName: String? = null,
    val joinPassword: String = "",
    val joinVenueId: Long? = null,
    val hasJoinedSession: Boolean = false,
    val manualLat: String = "",
    val manualLng: String = "",
)

private fun computeVotingRoundKey(s: VotingStateResponse): String {
    val sess = s.session
    if (sess == null || sess.status != "active") return ""
    val sessionPart = listOfNotNull(sess.id?.toString(), sess.playlistId).firstOrNull() ?: "active"
    val ids = s.candidates.orEmpty().map { it.id }.sorted().joinToString("|")
    return "$sessionPart|$ids"
}

class VenueViewModel(application: Application) : AndroidViewModel(application) {

    private val tokenStore = PatronTokenStore(application)
    private var token: String? = tokenStore.getToken()
    private val api = ApiClient.create(tokenProvider = { token })

    private val _ui = MutableStateFlow(VenueUiState())
    val ui: StateFlow<VenueUiState> = _ui.asStateFlow()

    private var pollJob: Job? = null

    init {
        if (token != null) {
            _ui.update { it.copy(patronEmail = "signed in", loading = false) }
        } else {
            _ui.update { it.copy(loading = false) }
        }
    }

    fun login(email: String, password: String) {
        viewModelScope.launch {
            _ui.update { it.copy(authLoading = true, authError = null) }
            try {
                val res = api.auth.login(LoginRequest(email.trim(), password))
                token = res.token
                tokenStore.saveToken(res.token)
                _ui.update {
                    it.copy(
                        patronEmail = res.user?.email ?: email,
                        authLoading = false,
                        hasJoinedSession = false,
                        joinedVenueName = null,
                    )
                }
            } catch (e: Exception) {
                _ui.update { it.copy(authLoading = false, authError = e.message ?: "Login failed") }
            }
        }
    }

    fun register(email: String, password: String) {
        viewModelScope.launch {
            _ui.update { it.copy(authLoading = true, authError = null) }
            try {
                val res = api.auth.register(RegisterRequest(email.trim(), password))
                token = res.token
                tokenStore.saveToken(res.token)
                _ui.update {
                    it.copy(
                        patronEmail = res.user?.email ?: email,
                        authLoading = false,
                        hasJoinedSession = false,
                    )
                }
            } catch (e: Exception) {
                _ui.update { it.copy(authLoading = false, authError = e.message ?: "Register failed") }
            }
        }
    }

    fun logout() {
        token = null
        tokenStore.clear()
        pollJob?.cancel()
        _ui.value = VenueUiState(loading = false)
    }

    @SuppressLint("MissingPermission")
    fun useDeviceLocation() {
        val lm = getApplication<Application>().getSystemService(Context.LOCATION_SERVICE) as LocationManager
        val loc = lm.getLastKnownLocation(LocationManager.NETWORK_PROVIDER)
            ?: lm.getLastKnownLocation(LocationManager.GPS_PROVIDER)
        if (loc != null) {
            _ui.update {
                it.copy(
                    manualLat = loc.latitude.toString(),
                    manualLng = loc.longitude.toString(),
                )
            }
        } else {
            _ui.update { it.copy(discoverError = "No last known location — enter coordinates manually") }
        }
    }

    fun setJoinPassword(value: String) {
        _ui.update { it.copy(joinPassword = value) }
    }

    fun setManualLat(value: String) {
        _ui.update { it.copy(manualLat = value) }
    }

    fun setManualLng(value: String) {
        _ui.update { it.copy(manualLng = value) }
    }

    fun loadNearby() {
        viewModelScope.launch {
            val lat = _ui.value.manualLat.toDoubleOrNull()
            val lng = _ui.value.manualLng.toDoubleOrNull()
            if (lat == null || lng == null) {
                _ui.update { it.copy(discoverError = "Enter valid latitude and longitude") }
                return@launch
            }
            _ui.update { it.copy(discoverLoading = true, discoverError = null) }
            try {
                val res = api.venue.nearby(lat, lng)
                _ui.update {
                    it.copy(
                        nearbySessions = res.sessions.orEmpty(),
                        discoverLoading = false,
                    )
                }
            } catch (e: Exception) {
                _ui.update {
                    it.copy(discoverLoading = false, discoverError = e.message ?: "Could not load nearby sessions")
                }
            }
        }
    }

    fun prepareJoin(venueId: Long) {
        _ui.update { it.copy(joinVenueId = venueId, joinPassword = "", discoverError = null) }
    }

    fun cancelJoin() {
        _ui.update { it.copy(joinVenueId = null, joinPassword = "") }
    }

    fun confirmJoin() {
        val venueId = _ui.value.joinVenueId ?: return
        viewModelScope.launch {
            _ui.update { it.copy(discoverLoading = true, discoverError = null) }
            try {
                val pwd = _ui.value.joinPassword.takeIf { it.isNotBlank() }
                val res = api.venue.join(venueId, JoinRequest(joinPassword = pwd))
                _ui.update {
                    it.copy(
                        hasJoinedSession = true,
                        joinedVenueName = res.venue?.name,
                        joinVenueId = null,
                        discoverLoading = false,
                    )
                }
                startPolling()
            } catch (e: Exception) {
                _ui.update {
                    it.copy(discoverLoading = false, discoverError = e.message ?: "Join failed")
                }
            }
        }
    }

    fun startPolling() {
        pollJob?.cancel()
        pollJob = viewModelScope.launch {
            while (isActive) {
                try {
                    val s = api.voting.getState()
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
                            val pl = api.voting.getPlaylistOverview()
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
                api.voting.vote(VoteRequest(trackId))
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
