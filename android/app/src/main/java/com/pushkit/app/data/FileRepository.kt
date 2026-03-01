package com.pushkit.app.data

import com.pushkit.app.data.api.PushKitApi
import com.pushkit.app.data.model.DownloadResponse
import com.pushkit.app.data.model.FileListResponse

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
}
