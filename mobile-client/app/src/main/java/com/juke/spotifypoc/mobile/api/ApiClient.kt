package com.juke.spotifypoc.mobile.api

import com.juke.spotifypoc.mobile.BuildConfig
import okhttp3.Interceptor
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory
import java.util.concurrent.TimeUnit

object ApiClient {
    fun create(
        baseUrl: String = BuildConfig.API_BASE_URL,
        tokenProvider: () -> String? = { null },
    ): JukeApis {
        val log = HttpLoggingInterceptor().apply {
            level = HttpLoggingInterceptor.Level.BASIC
        }
        val authInterceptor = Interceptor { chain ->
            val token = tokenProvider()
            val request = if (!token.isNullOrBlank()) {
                chain.request().newBuilder()
                    .header("Authorization", "Bearer $token")
                    .build()
            } else {
                chain.request()
            }
            chain.proceed(request)
        }
        val client = OkHttpClient.Builder()
            .addInterceptor(authInterceptor)
            .addInterceptor(log)
            .connectTimeout(30, TimeUnit.SECONDS)
            .readTimeout(30, TimeUnit.SECONDS)
            .build()

        val retrofit = Retrofit.Builder()
            .baseUrl(baseUrl)
            .client(client)
            .addConverterFactory(GsonConverterFactory.create())
            .build()

        return JukeApis(
            voting = retrofit.create(VotingApi::class.java),
            auth = retrofit.create(AuthApi::class.java),
            venue = retrofit.create(VenueApi::class.java),
            paidSkip = retrofit.create(PaidSkipApi::class.java),
        )
    }
}
