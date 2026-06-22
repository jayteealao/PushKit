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

@Serializable
data class UploadInitRequest(
    @SerialName("filename") val filename: String,
    @SerialName("contentType") val contentType: String,
    @SerialName("sizeBytes") val sizeBytes: Long,
    @SerialName("sha256") val sha256: String? = null
)

@Serializable
data class UploadInitResponse(
    @SerialName("fileId") val fileId: String,
    @SerialName("s3Key") val s3Key: String,
    @SerialName("presignedPutUrl") val presignedPutUrl: String,
    @SerialName("expiresInSeconds") val expiresInSeconds: Int,
    // Headers the server signed into the presigned PUT (notably Content-Type) — replay verbatim.
    @SerialName("requiredHeaders") val requiredHeaders: Map<String, String> = emptyMap()
)

@Serializable
data class UploadCompleteRequest(
    @SerialName("fileId") val fileId: String,
    @SerialName("sizeBytes") val sizeBytes: Long? = null,
    @SerialName("sha256") val sha256: String? = null,
    @SerialName("tags") val tags: Map<String, String>? = null
)
