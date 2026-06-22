package com.pushkit.app.data

import com.pushkit.app.data.api.PushKitApi
import com.pushkit.app.data.model.DownloadResponse
import com.pushkit.app.data.model.FileItem
import com.pushkit.app.data.model.FileListResponse
import com.pushkit.app.data.model.UploadCompleteRequest
import com.pushkit.app.data.model.UploadInitRequest
import com.pushkit.app.data.model.UploadInitResponse

class FileRepository(private val api: PushKitApi) {

    suspend fun listFiles(
        cursor: String? = null,
        search: String? = null,
        sort: String = "created_at",
        order: String = "desc"
    ): Result<FileListResponse> = runCatching {
        api.listFiles(cursor = cursor, search = search, sort = sort, order = order)
    }

    suspend fun getDownloadUrl(fileId: String): Result<DownloadResponse> = runCatching {
        api.getDownloadUrl(fileId)
    }

    suspend fun initUpload(request: UploadInitRequest): Result<UploadInitResponse> = runCatching {
        api.initUpload(request)
    }

    suspend fun completeUpload(request: UploadCompleteRequest): Result<FileItem> = runCatching {
        api.completeUpload(request)
    }

    /** Best-effort delete used to clean up an orphaned INITIATED row when an upload fails. */
    suspend fun deleteFile(fileId: String): Result<Unit> = runCatching {
        val response = api.deleteFile(fileId)
        if (!response.isSuccessful) {
            error("delete failed: HTTP ${response.code()}")
        }
    }
}
