package com.pushkit.app.data.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class FileItem(
    val id: String,
    @SerialName("originalFilename") val originalFilename: String,
    @SerialName("contentType") val contentType: String,
    @SerialName("sizeBytes") val sizeBytes: Long? = null,
    @SerialName("createdAt") val createdAt: String,
    val status: String
)

@Serializable
data class FileListResponse(
    val items: List<FileItem>,
    @SerialName("nextCursor") val nextCursor: String? = null
)

@Serializable
data class DownloadResponse(
    @SerialName("presignedGetUrl") val presignedGetUrl: String,
    @SerialName("expiresInSeconds") val expiresInSeconds: Int
)
