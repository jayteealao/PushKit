package com.pushkit.app.ui.files

import android.app.DownloadManager
import android.content.Context
import android.net.Uri
import android.os.Environment
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.work.WorkInfo
import androidx.work.WorkManager
import com.pushkit.app.data.FileRepository
import com.pushkit.app.data.upload.UploadFileSource
import com.pushkit.app.data.upload.UploadWorker
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.filterNotNull
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class FileListViewModel(
    private val repository: FileRepository,
    private val downloadManager: DownloadManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(FileListUiState())
    val uiState: StateFlow<FileListUiState> = _uiState.asStateFlow()

    private var searchJob: Job? = null

    init {
        loadFiles(refresh = true)
    }

    fun refresh() {
        loadFiles(refresh = true)
    }

    fun loadMore() {
        val cursor = _uiState.value.nextCursor ?: return
        if (_uiState.value.isLoading) return
        loadFiles(refresh = false, cursor = cursor)
    }

    fun search(query: String) {
        _uiState.update { it.copy(searchQuery = query) }
        searchJob?.cancel()
        searchJob = viewModelScope.launch {
            delay(300) // debounce
            loadFiles(refresh = true)
        }
    }

    fun setSort(field: String) {
        val currentOrder = _uiState.value.sortOrder
        val newOrder = if (_uiState.value.sortField == field) {
            if (currentOrder == "desc") "asc" else "desc"
        } else {
            "desc"
        }
        _uiState.update { it.copy(sortField = field, sortOrder = newOrder) }
        loadFiles(refresh = true)
    }

    fun download(fileId: String, context: Context) {
        viewModelScope.launch {
            repository.getDownloadUrl(fileId)
                .onSuccess { response ->
                    val request = DownloadManager.Request(Uri.parse(response.presignedGetUrl))
                        .setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED)
                        .setDestinationInExternalPublicDir(
                            Environment.DIRECTORY_DOWNLOADS,
                            getUniqueFilename(context, fileId)
                        )
                    downloadManager.enqueue(request)
                    _uiState.update { it.copy(downloadMessage = "Download started") }
                    delay(2000)
                    _uiState.update { it.copy(downloadMessage = null) }
                }
                .onFailure { error ->
                    _uiState.update { it.copy(error = "Download failed: ${error.message}") }
                }
        }
    }

    /**
     * Copies the picked file into app cache (off the main thread), enqueues the foreground upload
     * worker, then observes it to completion and refreshes the list so the new file appears.
     */
    fun enqueueUpload(uri: Uri, context: Context) {
        val appContext = context.applicationContext
        viewModelScope.launch {
            _uiState.update {
                it.copy(isUploading = true, uploadMessage = "Preparing upload…", error = null)
            }

            val cached = withContext(Dispatchers.IO) {
                UploadFileSource.copyToCache(appContext.contentResolver, uri, appContext.cacheDir)
            }.getOrElse { err ->
                _uiState.update {
                    it.copy(
                        isUploading = false,
                        uploadMessage = null,
                        error = "Upload failed: ${err.message ?: "could not read file"}"
                    )
                }
                return@launch
            }

            val request = UploadWorker.buildRequest(cached)

            val workManager = WorkManager.getInstance(appContext)
            workManager.enqueue(request)
            _uiState.update { it.copy(uploadMessage = "Uploading ${cached.filename}…") }

            val finished = workManager.getWorkInfoByIdFlow(request.id)
                .filterNotNull()
                .first { it.state.isFinished }

            if (finished.state == WorkInfo.State.SUCCEEDED) {
                _uiState.update { it.copy(isUploading = false, uploadMessage = "Upload complete") }
                loadFiles(refresh = true)
            } else {
                _uiState.update {
                    it.copy(isUploading = false, uploadMessage = null, error = "Upload failed")
                }
            }
        }
    }

    fun dismissError() {
        _uiState.update { it.copy(error = null) }
    }

    private fun loadFiles(refresh: Boolean, cursor: String? = null) {
        viewModelScope.launch {
            _uiState.update {
                if (refresh) it.copy(isRefreshing = true, error = null)
                else it.copy(isLoading = true, error = null)
            }

            val state = _uiState.value
            repository.listFiles(
                cursor = cursor,
                search = state.searchQuery.ifBlank { null },
                sort = state.sortField,
                order = state.sortOrder
            ).onSuccess { response ->
                _uiState.update {
                    val newFiles = if (refresh) response.items else it.files + response.items
                    it.copy(
                        files = newFiles,
                        nextCursor = response.nextCursor,
                        hasMore = response.nextCursor != null,
                        isLoading = false,
                        isRefreshing = false
                    )
                }
            }.onFailure { error ->
                val message = when {
                    error.message?.contains("401") == true -> "Invalid API key"
                    error.message?.contains("500") == true -> "Server error"
                    else -> error.message ?: "Unknown error"
                }
                _uiState.update {
                    it.copy(
                        error = message,
                        isLoading = false,
                        isRefreshing = false
                    )
                }
            }
        }
    }

    private fun getUniqueFilename(context: Context, fileId: String): String {
        // Find the file in our list to get the original filename.
        val file = _uiState.value.files.find { it.id == fileId }
        return file?.originalFilename ?: "$fileId.bin"
    }
}
