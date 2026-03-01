package com.pushkit.app.data.api

import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import com.pushkit.app.data.CredentialStore
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.Retrofit
import java.util.concurrent.TimeUnit

object RetrofitProvider {

    private val json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
    }

    fun create(credentialStore: CredentialStore): PushKitApi {
        val baseUrl = credentialStore.apiUrl.let {
            if (it.endsWith("/")) it else "$it/"
        }

        val client = OkHttpClient.Builder()
            .addInterceptor(ApiKeyInterceptor(credentialStore))
            .addInterceptor(HttpLoggingInterceptor().apply {
                level = HttpLoggingInterceptor.Level.BODY
            })
            .connectTimeout(30, TimeUnit.SECONDS)
            .readTimeout(30, TimeUnit.SECONDS)
            .build()

        return Retrofit.Builder()
            .baseUrl(baseUrl)
            .client(client)
            .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(PushKitApi::class.java)
    }
}
