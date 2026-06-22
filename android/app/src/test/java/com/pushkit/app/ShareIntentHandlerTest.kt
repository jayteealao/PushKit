package com.pushkit.app

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.util.Log
import androidx.test.core.app.ApplicationProvider
import androidx.work.Configuration
import androidx.work.WorkInfo
import androidx.work.WorkManager
import androidx.work.testing.WorkManagerTestInitHelper
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import java.io.ByteArrayInputStream

/**
 * Verifies share-intent routing: ACTION_SEND with a content:// URI enqueues exactly one upload
 * request; non-matching intents are no-ops.
 *
 * Tests invoke [enqueueFromShareIntent] directly (the package-level helper that MainActivity
 * delegates to). The copy-to-cache step runs against the Robolectric ContentResolver (stream
 * registered via shadowOf) and WorkManager is initialised via WorkManagerTestInitHelper so the
 * in-process instance can be queried synchronously.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class ShareIntentHandlerTest {

    private lateinit var context: Context

    @Before
    fun setUp() {
        context = ApplicationProvider.getApplicationContext()
        WorkManagerTestInitHelper.initializeTestWorkManager(
            context,
            Configuration.Builder().setMinimumLoggingLevel(Log.DEBUG).build()
        )
    }

    @After
    fun tearDown() {
        WorkManager.getInstance(context).cancelAllWork()
    }

    @Test
    fun actionSend_withUri_enqueuesWork() {
        val content = "hello share".toByteArray()
        val uri = Uri.parse("content://com.test.provider/files/hello.txt")
        shadowOf(context.contentResolver).registerInputStream(uri, ByteArrayInputStream(content))

        val intent = Intent(Intent.ACTION_SEND).apply {
            type = "text/plain"
            putExtra(Intent.EXTRA_STREAM, uri)
        }
        enqueueFromShareIntent(intent, context)

        val infos = WorkManager.getInstance(context)
            .getWorkInfosByTag(TAG_SHARE_UPLOAD)
            .get()
        assertEquals(
            "expected exactly one enqueued work request from a valid share intent",
            1,
            infos.size
        )
        assertEquals(WorkInfo.State.ENQUEUED, infos.first().state)
    }

    @Test
    fun actionSend_withNullExtraStream_isNoOp() {
        val intent = Intent(Intent.ACTION_SEND).apply { type = "text/plain" }
        enqueueFromShareIntent(intent, context)

        val infos = WorkManager.getInstance(context)
            .getWorkInfosByTag(TAG_SHARE_UPLOAD)
            .get()
        assertEquals("null EXTRA_STREAM must not enqueue any work", 0, infos.size)
    }

    @Test
    fun nonSendIntent_isNoOp() {
        enqueueFromShareIntent(Intent(Intent.ACTION_VIEW), context)

        val infos = WorkManager.getInstance(context)
            .getWorkInfosByTag(TAG_SHARE_UPLOAD)
            .get()
        assertEquals("non-ACTION_SEND intents must not enqueue any work", 0, infos.size)
    }
}
