package com.pushkit.app.data.upload

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.os.Build
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import androidx.work.BackoffPolicy
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ForegroundInfo
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequest
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkRequest
import androidx.work.WorkerParameters
import androidx.work.workDataOf
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
open class UploadWorker(
    appContext: Context,
    params: WorkerParameters
) : CoroutineWorker(appContext, params) {

    override suspend fun doWork(): Result {
        val path = inputData.getString(KEY_CACHE_PATH) ?: return Result.failure()
        val filename = inputData.getString(KEY_FILENAME) ?: UploadDefaults.FILENAME
        val contentType = inputData.getString(KEY_CONTENT_TYPE) ?: UploadDefaults.CONTENT_TYPE
        val size = inputData.getLong(KEY_SIZE, -1L)

        val cacheFile = File(path)
        // Terminal guard: file already gone or size invalid — no retry can help.
        if (!cacheFile.exists() || size <= 0L) {
            cacheFile.delete()
            return Result.failure()
        }

        runCatching { setForeground(buildForegroundInfo(filename)) }

        val orchestrator = buildOrchestrator()
        val cached = CachedUpload(cacheFile, filename, contentType, size)

        val result = orchestrator.upload(cached).fold(
            onSuccess = {
                notifyResult(filename, success = true)
                // Terminal success: delete the cache file now.
                cacheFile.delete()
                Result.success()
            },
            onFailure = {
                notifyResult(filename, success = false)
                if (runAttemptCount + 1 < MAX_ATTEMPTS) {
                    // Not yet exhausted: retry without deleting so the next attempt can read the file.
                    Result.retry()
                } else {
                    // Exhausted all attempts: terminal failure, delete now.
                    cacheFile.delete()
                    Result.failure()
                }
            }
        )

        return result
    }

    /**
     * Overridable seam for tests: return a fake [UploadOrchestrator] in a subclass or
     * [TestListenableWorkerBuilder] subclass without needing a real device or EncryptedSharedPrefs.
     * Production code creates the orchestrator inline from `applicationContext`.
     */
    internal open fun buildOrchestrator(): UploadOrchestrator {
        val repository = FileRepository(RetrofitProvider.create(CredentialStore(applicationContext)))
        return UploadOrchestrator(repository, putClient())
    }

    override suspend fun getForegroundInfo(): ForegroundInfo =
        buildForegroundInfo(inputData.getString(KEY_FILENAME) ?: UploadDefaults.FILENAME)

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
        // POST_NOTIFICATIONS is runtime-revocable on API 33+; skip the terminal notification when it
        // isn't granted (the foreground progress notification is exempt and still shows regardless).
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(
                applicationContext,
                Manifest.permission.POST_NOTIFICATIONS
            ) != PackageManager.PERMISSION_GRANTED
        ) {
            return
        }
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

        private const val MAX_ATTEMPTS = 3

        /** Tag applied to every WorkManager request enqueued from a share intent. */
        const val TAG_SHARE_UPLOAD = "share_upload"

        /**
         * Single factory for all [OneTimeWorkRequest] enqueues of [UploadWorker].
         * Encapsulates the network constraint, exponential back-off, input data keys, and optional
         * tag — so all three call sites (share intent, FAB picker, and any future path) stay in sync.
         */
        fun buildRequest(cached: CachedUpload, tag: String? = null): OneTimeWorkRequest {
            val builder = OneTimeWorkRequestBuilder<UploadWorker>()
                .setConstraints(
                    Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build()
                )
                .setBackoffCriteria(
                    BackoffPolicy.EXPONENTIAL,
                    WorkRequest.MIN_BACKOFF_MILLIS,
                    TimeUnit.MILLISECONDS
                )
                .setInputData(
                    workDataOf(
                        KEY_CACHE_PATH to cached.file.absolutePath,
                        KEY_FILENAME to cached.filename,
                        KEY_CONTENT_TYPE to cached.contentType,
                        KEY_SIZE to cached.sizeBytes
                    )
                )
            if (tag != null) builder.addTag(tag)
            return builder.build()
        }
    }
}
