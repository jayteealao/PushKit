package com.pushkit.app.data.upload

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import androidx.work.ListenableWorker
import androidx.work.WorkerFactory
import androidx.work.WorkerParameters
import androidx.work.testing.TestListenableWorkerBuilder
import androidx.work.workDataOf
import com.pushkit.app.data.FileRepository
import com.pushkit.app.data.model.FileItem
import kotlinx.coroutines.runBlocking
import okhttp3.OkHttpClient
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.io.File

/**
 * Covers the worker shell: input-data validation, terminal failure mapping, foreground info, and
 * the retry / exhaustion logic (H1 / H3).
 *
 * The init->PUT->complete success/retry network logic lives in [UploadOrchestrator] and is covered
 * by UploadOrchestratorTest (the worker builds an EncryptedSharedPreferences-backed CredentialStore
 * that does not run under Robolectric, so the network paths are tested at the orchestrator seam).
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class UploadWorkerTest {

    private val context = ApplicationProvider.getApplicationContext<Context>()

    // ---------------------------------------------------------------------------
    // Helpers: fake orchestrator + injectable worker subclass
    // ---------------------------------------------------------------------------

    /**
     * Fake orchestrator: overrides [upload] to either succeed or fail without touching the network
     * or EncryptedSharedPreferences. The repo/client passed to the constructor are never called.
     */
    private class FakeOrchestrator(private val succeed: Boolean) : UploadOrchestrator(
        FileRepository(
            retrofit2.Retrofit.Builder()
                .baseUrl("http://localhost/")
                .build()
                .create(com.pushkit.app.data.api.PushKitApi::class.java)
        ),
        OkHttpClient()
    ) {
        override suspend fun upload(cached: CachedUpload): Result<FileItem> =
            if (succeed) Result.success(
                FileItem(
                    id = "fake",
                    originalFilename = cached.filename,
                    contentType = cached.contentType,
                    sizeBytes = cached.sizeBytes,
                    createdAt = "2026-01-01T00:00:00Z",
                    status = "UPLOADED"
                )
            )
            else Result.failure(Exception("forced failure"))
    }

    /**
     * Worker subclass that injects a [FakeOrchestrator] via the [buildOrchestrator] seam (H3).
     * This allows testing retry/failure paths without a real device, CredentialStore, or network.
     */
    private class InjectableUploadWorker(
        appContext: Context,
        params: WorkerParameters,
        private val fakeOrchestrator: FakeOrchestrator
    ) : UploadWorker(appContext, params) {
        override fun buildOrchestrator(): UploadOrchestrator = fakeOrchestrator
    }

    private fun buildWorkerWithFake(
        cacheFile: File,
        succeed: Boolean,
        attemptCount: Int = 0
    ): InjectableUploadWorker {
        val fake = FakeOrchestrator(succeed)
        return TestListenableWorkerBuilder<InjectableUploadWorker>(context)
            .setWorkerFactory(object : WorkerFactory() {
                override fun createWorker(
                    appContext: Context,
                    workerClassName: String,
                    workerParameters: WorkerParameters
                ): ListenableWorker = InjectableUploadWorker(appContext, workerParameters, fake)
            })
            .setInputData(
                workDataOf(
                    UploadWorker.KEY_CACHE_PATH to cacheFile.absolutePath,
                    UploadWorker.KEY_FILENAME to "test.bin",
                    UploadWorker.KEY_CONTENT_TYPE to "application/octet-stream",
                    UploadWorker.KEY_SIZE to cacheFile.length()
                )
            )
            .setRunAttemptCount(attemptCount)
            .build()
    }

    // ---------------------------------------------------------------------------
    // Original validation tests
    // ---------------------------------------------------------------------------

    @Test
    fun missingCacheFile_returnsFailure() {
        val worker = TestListenableWorkerBuilder<UploadWorker>(context)
            .setInputData(
                workDataOf(
                    UploadWorker.KEY_CACHE_PATH to "/does/not/exist.bin",
                    UploadWorker.KEY_SIZE to 10L
                )
            )
            .build()

        val result = runBlocking { worker.doWork() }
        assertEquals(ListenableWorker.Result.failure(), result)
    }

    @Test
    fun nonPositiveSize_returnsFailure() {
        val real = File.createTempFile("present", ".bin")
        real.writeText("x")
        try {
            val worker = TestListenableWorkerBuilder<UploadWorker>(context)
                .setInputData(workDataOf(UploadWorker.KEY_CACHE_PATH to real.absolutePath))
                .build() // KEY_SIZE absent -> defaults to -1 -> failure
            val result = runBlocking { worker.doWork() }
            assertEquals(ListenableWorker.Result.failure(), result)
        } finally {
            real.delete()
        }
    }

    @Test
    fun getForegroundInfo_usesProgressNotificationId() {
        val worker = TestListenableWorkerBuilder<UploadWorker>(context)
            .setInputData(workDataOf(UploadWorker.KEY_FILENAME to "doc.pdf"))
            .build()
        val info = runBlocking { worker.getForegroundInfo() }
        assertEquals(UploadNotifications.PROGRESS_NOTIFICATION_ID, info.notificationId)
    }

    // ---------------------------------------------------------------------------
    // H1 / TST-4: retry-path and exhaustion tests
    // ---------------------------------------------------------------------------

    /**
     * On attempt 0 of 3, an orchestrator failure must produce [Result.retry] — NOT [Result.failure].
     * Critically, the cache file must still exist after the retry result so the next attempt can
     * read it (locks the H1 fix: no unconditional finally-delete).
     */
    @Test
    fun orchestratorFailure_earlyAttempt_returnsRetry_andCacheFileExists() {
        val cacheFile = File.createTempFile("upload_", ".bin").apply { writeText("data") }
        try {
            val worker = buildWorkerWithFake(cacheFile, succeed = false, attemptCount = 0)
            val result = runBlocking { worker.doWork() }
            assertEquals(
                "attempt 0/3 should retry, not fail",
                ListenableWorker.Result.retry(),
                result
            )
            assertTrue(
                "cache file must still exist after a retry result so the next attempt can read it",
                cacheFile.exists()
            )
        } finally {
            cacheFile.delete()
        }
    }

    /**
     * On the final allowed attempt (runAttemptCount = MAX_ATTEMPTS - 1 = 2), an orchestrator
     * failure must produce [Result.failure] and the cache file must be deleted (terminal cleanup).
     */
    @Test
    fun orchestratorFailure_lastAttempt_returnsFailure_andCacheFileDeleted() {
        val cacheFile = File.createTempFile("upload_", ".bin").apply { writeText("data") }
        // MAX_ATTEMPTS = 3; last attempt index = 2
        val worker = buildWorkerWithFake(cacheFile, succeed = false, attemptCount = 2)
        val result = runBlocking { worker.doWork() }
        assertEquals(
            "final attempt should return failure",
            ListenableWorker.Result.failure(),
            result
        )
        assertTrue(
            "cache file must be deleted after the final failed attempt",
            !cacheFile.exists()
        )
    }

    /**
     * On success the cache file must be deleted (terminal cleanup) and [Result.success] returned.
     */
    @Test
    fun orchestratorSuccess_returnsSuccess_andCacheFileDeleted() {
        val cacheFile = File.createTempFile("upload_", ".bin").apply { writeText("data") }
        val worker = buildWorkerWithFake(cacheFile, succeed = true, attemptCount = 0)
        val result = runBlocking { worker.doWork() }
        assertEquals(ListenableWorker.Result.success(), result)
        assertTrue("cache file must be deleted on success", !cacheFile.exists())
    }
}
