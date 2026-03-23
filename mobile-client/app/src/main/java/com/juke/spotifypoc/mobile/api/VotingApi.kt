package com.juke.spotifypoc.mobile.api

import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST

interface VotingApi {
    @GET("api/voting/state")
    suspend fun getState(): VotingStateResponse

    @POST("api/voting/vote")
    suspend fun vote(@Body body: VoteRequest)

    @GET("api/voting/playlist-overview")
    suspend fun getPlaylistOverview(): PlaylistOverviewResponse
}
