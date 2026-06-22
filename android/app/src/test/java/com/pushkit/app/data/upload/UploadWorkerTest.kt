package com.pushkit.app.data.upload

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import androidx.work.ListenableWorker
import androidx.work.testing.TestListenableWorkerBuilder
import androidx.work.workDataOf
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.io.File

/**
 * Covers the worker shell: input-data validation, terminal failure mapping, and foreground info.
 * The init->PUT->complete success/retry network logic lives in [UploadOrchestrator] and is covered
 * by UploadOrchestratorTest (the worker builds an EncryptedSharedPreferences-backed CredentialStore
 * that does not run under Robolectric, so the network paths are tested at the orchestrator seam).
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class UploadWorkerTest {

    private val context = ApplicationProvider.getApplicationContext<Context>()

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
}
