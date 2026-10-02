package com.juke.spotifypoc.mobile.api

import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST

interface PaidSkipApi {
    @GET("api/paid-skip/config")
    suspend fun config(): PaidSkipConfigResponse

    @POST("api/paid-skip/checkout")
    suspend fun checkout(@Body body: PaidSkipCheckoutRequest): PaidSkipCheckoutResponse
}

data class PaidSkipConfigResponse(
    @com.google.gson.annotations.SerializedName("price_usd") val priceUsd: String = "1.00",
    val currency: String = "usd",
    @com.google.gson.annotations.SerializedName("stripe_enabled") val stripeEnabled: Boolean = false,
    @com.google.gson.annotations.SerializedName("product_name") val productName: String = "",
)

data class PaidSkipCheckoutRequest(
    @com.google.gson.annotations.SerializedName("track_id") val trackId: String,
)

data class PaidSkipCheckoutResponse(
    @com.google.gson.annotations.SerializedName("checkout_url") val checkoutUrl: String = "",
    @com.google.gson.annotations.SerializedName("session_id") val sessionId: String = "",
    @com.google.gson.annotations.SerializedName("price_usd") val priceUsd: String = "",
)
