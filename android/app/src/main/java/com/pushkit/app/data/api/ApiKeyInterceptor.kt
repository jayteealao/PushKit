package com.pushkit.app.data.api

import com.pushkit.app.data.CredentialStore
import okhttp3.Interceptor
import okhttp3.Response

class ApiKeyInterceptor(private val credentialStore: CredentialStore) : Interceptor {

    override fun intercept(chain: Interceptor.Chain): Response {
        val apiKey = credentialStore.apiKey
        if (apiKey.isBlank()) {
            throw IllegalStateException("API key not configured")
        }

        val request = chain.request().newBuilder()
            .addHeader("X-API-Key", apiKey)
            .build()

        return chain.proceed(request)
    }
}
