package com.pushkit.app.data

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKeys

class CredentialStore(context: Context) {

    private val prefs: SharedPreferences = EncryptedSharedPreferences.create(
        "pushkit_credentials",
        MasterKeys.getOrCreate(MasterKeys.AES256_GCM_SPEC),
        context,
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
    )

    var apiUrl: String
        get() = prefs.getString(KEY_API_URL, "") ?: ""
        set(value) = prefs.edit().putString(KEY_API_URL, value.trimEnd('/')).apply()

    var apiKey: String
        get() = prefs.getString(KEY_API_KEY, "") ?: ""
        set(value) = prefs.edit().putString(KEY_API_KEY, value.trim()).apply()

    val isConfigured: Boolean
        get() = apiUrl.isNotBlank() && apiKey.isNotBlank()

    companion object {
        private const val KEY_API_URL = "api_url"
        private const val KEY_API_KEY = "api_key"
    }
}
