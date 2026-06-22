package com.pushkit.app.data.upload

import com.pushkit.app.data.FileRepository
import com.pushkit.app.data.model.FileItem
import com.pushkit.app.data.model.UploadCompleteRequest
import com.pushkit.app.data.model.UploadInitRequest
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.asRequestBody
import java.io.IOException

/**
 * The upload orchestration: `init` → presigned S3 PUT → `complete`. Kept as a plain class with
 * injected seams (a [FileRepository] and a dedicated PUT [OkHttpClient]) so it can be unit-tested
 * against a `MockWebServer` without a device.
 *
 * The PUT client is deliberately separate from the app's Retrofit client: it must NOT carry the
 * `X-API-Key` interceptor (the presigned URL is self-authenticated) nor the BODY logging
 * interceptor (which would consume the streamed body). The body is the cache [File] via
 * `asRequestBody`, which is repeatable (retry-safe) and reports an exact `Content-Length` (so the
 * request is not sent with `Transfer-Encoding: chunked`, which S3 presigned PUTs reject).
 */
class UploadOrchestrator(
    private val repository: FileRepository,
    private val putClient: OkHttpClient
) {

    suspend fun upload(cached: CachedUpload): Result<FileItem> {
        val init = repository.initUpload(
            UploadInitRequest(
                filename = cached.filename,
                contentType = cached.contentType,
                sizeBytes = cached.sizeBytes
            )
        ).getOrElse { return Result.failure(it) }

        // The server signs Content-Type into the presigned URL and echoes the exact value back.
        // Replay it verbatim; fall back to what we sent only if the map is empty.
        val signedContentType = init.requiredHeaders["Content-Type"] ?: cached.contentType

        val putResult = runCatching { putObject(init.presignedPutUrl, cached, signedContentType) }
        if (putResult.isFailure) {
            // Best-effort cleanup of the INITIATED orphan; ignore the cleanup result.
            repository.deleteFile(init.fileId)
            return Result.failure(putResult.exceptionOrNull() ?: IOException("upload PUT failed"))
        }

        return repository.completeUpload(
            UploadCompleteRequest(fileId = init.fileId, sizeBytes = cached.sizeBytes)
        ).onFailure { repository.deleteFile(init.fileId) }
    }

    private suspend fun putObject(url: String, cached: CachedUpload, contentType: String) =
        withContext(Dispatchers.IO) {
            val body = cached.file.asRequestBody(contentType.toMediaTypeOrNull())
            val request = Request.Builder()
                .url(url)
                .put(body)
                .header("Content-Type", contentType)
                .build()
            putClient.newCall(request).execute().use { response ->
                if (!response.isSuccessful) {
                    throw IOException("S3 PUT failed: HTTP ${response.code}")
                }
            }
        }
}
