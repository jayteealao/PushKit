package com.pushkit.app.ui.settings

import androidx.lifecycle.ViewModel
import com.pushkit.app.data.CredentialStore
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

data class SettingsUiState(
    val apiUrl: String = "",
    val apiKey: String = "",
    val error: String? = null,
    val isSaved: Boolean = false
)

class SettingsViewModel(private val credentialStore: CredentialStore) : ViewModel() {

    private val _uiState = MutableStateFlow(SettingsUiState(
        apiUrl = credentialStore.apiUrl,
        apiKey = credentialStore.apiKey
    ))
    val uiState: StateFlow<SettingsUiState> = _uiState.asStateFlow()

    fun updateApiUrl(url: String) {
        _uiState.update { it.copy(apiUrl = url, error = null, isSaved = false) }
    }

    fun updateApiKey(key: String) {
        _uiState.update { it.copy(apiKey = key, error = null, isSaved = false) }
    }

    fun save(): Boolean {
        val state = _uiState.value

        if (state.apiUrl.isBlank()) {
            _uiState.update { it.copy(error = "API URL is required") }
            return false
        }
        if (!state.apiUrl.startsWith("http://") && !state.apiUrl.startsWith("https://")) {
            _uiState.update { it.copy(error = "API URL must start with http:// or https://") }
            return false
        }
        if (state.apiKey.isBlank()) {
            _uiState.update { it.copy(error = "API key is required") }
            return false
        }

        credentialStore.apiUrl = state.apiUrl
        credentialStore.apiKey = state.apiKey
        _uiState.update { it.copy(isSaved = true, error = null) }
        return true
    }
}
