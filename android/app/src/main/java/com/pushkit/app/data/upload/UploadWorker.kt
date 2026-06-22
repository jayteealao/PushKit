package com.pushkit.app.data.upload

import android.content.Context
import android.content.pm.ServiceInfo
import android.os.Build
import androidx.core.app.NotificationManagerCompat
import androidx.work.CoroutineWorker
import androidx.work.ForegroundInfo
import androidx.work.WorkerParameters
import com.pushkit.app.data.CredentialStore
import com.pushkit.app.data.FileRepository
import com.pushkit.app.data.api.RetrofitProvider
import okhttp3.OkHttpClient
import java.io.File
import java.util.concurrent.TimeUnit

/**
 * Long-running foreground worker that uploads one already-cached file via
 * [UploadOrchestrator]. Running as a foreground (data-sync) service lets large uploads survive
 * memory pressure and the background execution limit. The worker builds its own dependencies from
 * `applicationContext` (no custom `WorkerFactory` needed): [CredentialStore] reads the stored
 * API URL/key, and a dedicated [OkHttpClient] performs the presigned S3 PUT.
 */
class UploadWorker(
    appContext: Context,
    params: WorkerParameters
) : CoroutineWorker(appContext, params) {

    override suspend fun doWork(): Result {
        val path = inputData.getString(KEY_CACHE_PATH) ?: return Result.failure()
        val filename = inputData.getString(KEY_FILENAME) ?: DEFAULT_FILENAME
        val contentType = inputData.getString(KEY_CONTENT_TYPE) ?: DEFAULT_CONTENT_TYPE
        val size = inputData.getLong(KEY_SIZE, -1L)

        val cacheFile = File(path)
        if (!cacheFile.exists() || size <= 0L) {
            cacheFile.delete()
            return Result.failure()
        }

        runCatching { setForeground(buildForegroundInfo(filename)) }

        val repository = FileRepository(RetrofitProvider.create(CredentialStore(applicationContext)))
        val orchestrator = UploadOrchestrator(repository, putClient())
        val cached = CachedUpload(cacheFile, filename, contentType, size)

        return try {
            orchestrator.upload(cached).fold(
                onSuccess = {
                    notifyResult(filename, success = true)
                    Result.success()
                },
                onFailure = {
                    notifyResult(filename, success = false)
                    if (runAttemptCount + 1 < MAX_ATTEMPTS) Result.retry() else Result.failure()
                }
            )
        } finally {
            cacheFile.delete()
        }
    }

    override suspend fun getForegroundInfo(): ForegroundInfo =
        buildForegroundInfo(inputData.getString(KEY_FILENAME) ?: DEFAULT_FILENAME)

    private fun buildForegroundInfo(filename: String): ForegroundInfo {
        val notification = UploadNotifications.progress(applicationContext, filename)
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            ForegroundInfo(
                UploadNotifications.PROGRESS_NOTIFICATION_ID,
                notification,
                ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC
            )
        } else {
            ForegroundInfo(UploadNotifications.PROGRESS_NOTIFICATION_ID, notification)
        }
    }

    private fun notifyResult(filename: String, success: Boolean) {
        val notification = UploadNotifications.terminal(
            applicationContext,
            title = if (success) "Upload complete" else "Upload failed",
            text = filename
        )
        runCatching {
            NotificationManagerCompat.from(applicationContext)
                .notify(UploadNotifications.RESULT_NOTIFICATION_ID, notification)
        }
    }

    /** No write timeout — uploads can be arbitrarily large (no size cap). */
    private fun putClient(): OkHttpClient =
        OkHttpClient.Builder()
            .connectTimeout(30, TimeUnit.SECONDS)
            .writeTimeout(0, TimeUnit.SECONDS)
            .readTimeout(60, TimeUnit.SECONDS)
            .build()

    companion object {
        const val KEY_CACHE_PATH = "cache_path"
        const val KEY_FILENAME = "filename"
        const val KEY_CONTENT_TYPE = "content_type"
        const val KEY_SIZE = "size"

        private const val DEFAULT_FILENAME = "upload.bin"
        private const val DEFAULT_CONTENT_TYPE = "application/octet-stream"
        private const val MAX_ATTEMPTS = 3
    }
}
