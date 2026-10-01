package com.juke.spotifypoc.mobile.api

import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.Path
import retrofit2.http.Query

interface VenueApi {
    @GET("api/venues/nearby")
    suspend fun nearby(
        @Query("lat") lat: Double,
        @Query("lng") lng: Double,
        @Query("radius_m") radiusM: Int = 5000,
    ): NearbyResponse

    @POST("api/venues/{id}/join")
    suspend fun join(
        @Path("id") venueId: Long,
        @Body body: JoinRequest,
    ): JoinResponse
}
