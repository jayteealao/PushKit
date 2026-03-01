package com.pushkit.app.data.api

import com.pushkit.app.data.model.DownloadResponse
import com.pushkit.app.data.model.FileListResponse
import retrofit2.http.GET
import retrofit2.http.Path
import retrofit2.http.Query

interface PushKitApi {

    @GET("v1/files")
    suspend fun listFiles(
        @Query("cursor") cursor: String? = null,
        @Query("limit") limit: Int = 20,
        @Query("q") search: String? = null,
        @Query("sort") sort: String = "created_at",
        @Query("order") order: String = "desc"
    ): FileListResponse

    @GET("v1/files/{fileId}/download")
    suspend fun getDownloadUrl(
        @Path("fileId") fileId: String
    ): DownloadResponse
}
