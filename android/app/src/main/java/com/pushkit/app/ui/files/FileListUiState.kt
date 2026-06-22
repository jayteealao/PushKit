package com.pushkit.app.ui.files

import com.pushkit.app.data.model.FileItem

data class FileListUiState(
    val files: List<FileItem> = emptyList(),
    val isLoading: Boolean = false,
    val isRefreshing: Boolean = false,
    val error: String? = null,
    val nextCursor: String? = null,
    val searchQuery: String = "",
    val sortField: String = "created_at",
    val sortOrder: String = "desc",
    val hasMore: Boolean = false,
    val downloadMessage: String? = null,
    val isUploading: Boolean = false,
    val uploadMessage: String? = null
)
