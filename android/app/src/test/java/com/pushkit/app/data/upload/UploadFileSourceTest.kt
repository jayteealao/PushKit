package com.pushkit.app.data.upload

import android.content.Context
import android.net.Uri
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import java.io.ByteArrayInputStream

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class UploadFileSourceTest {

    private val app = ApplicationProvider.getApplicationContext<Context>()

    @Test
    fun copyToCache_streamsBytes_andMeasuresExactSize() {
        // 5 MB payload to exercise the chunked streaming copy (not a single in-memory read).
        val payload = ByteArray(5 * 1024 * 1024) { (it % 251).toByte() }
        val uri = Uri.parse("content://test/big")
        shadowOf(app.contentResolver).registerInputStream(uri, ByteArrayInputStream(payload))

        val result = UploadFileSource.copyToCache(app.contentResolver, uri, app.cacheDir)

        assertTrue(result.isSuccess)
        val cached = result.getOrThrow()
        // Size is measured from the bytes actually copied (resolves the null OpenableColumns.SIZE case).
        assertEquals(payload.size.toLong(), cached.sizeBytes)
        assertEquals(payload.size.toLong(), cached.file.length())
        // getType is null here, so the content type falls back to octet-stream.
        assertEquals("application/octet-stream", cached.contentType)
        cached.file.delete()
    }

    @Test
    fun copyToCache_unreadableUri_returnsFailure() {
        // No stream registered -> openInputStream yields nothing -> failure (not a crash).
        val uri = Uri.parse("content://test/missing")
        val result = UploadFileSource.copyToCache(app.contentResolver, uri, app.cacheDir)
        assertFalse(result.isSuccess)
    }
}
