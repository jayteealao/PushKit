package com.pushkit.app.data.api

import com.pushkit.app.data.model.DownloadResponse
import com.pushkit.app.data.model.FileItem
import com.pushkit.app.data.model.FileListResponse
import com.pushkit.app.data.model.UploadCompleteRequest
import com.pushkit.app.data.model.UploadInitRequest
import com.pushkit.app.data.model.UploadInitResponse
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
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

    @POST("v1/uploads/init")
    suspend fun initUpload(@Body request: UploadInitRequest): UploadInitResponse

    @POST("v1/uploads/complete")
    suspend fun completeUpload(@Body request: UploadCompleteRequest): FileItem

    @DELETE("v1/files/{fileId}")
    suspend fun deleteFile(@Path("fileId") fileId: String): Response<Unit>
}
