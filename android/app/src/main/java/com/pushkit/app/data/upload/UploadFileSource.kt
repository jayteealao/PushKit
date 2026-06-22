package com.pushkit.app.data.upload

import android.content.ContentResolver
import android.net.Uri
import android.provider.OpenableColumns
import java.io.File
import java.io.IOException

/** A picked file copied into app-private cache, ready for a deferred background upload. */
data class CachedUpload(
    val file: File,
    val filename: String,
    val contentType: String,
    val sizeBytes: Long
)

/**
 * Copies a picked `content://` URI into a private cache file so a deferred WorkManager worker can
 * read it even after the transient picker grant is gone (SAF grants are not persisted here, and
 * Photo Picker grants are session-scoped). The copy streams in fixed-size chunks — it never buffers
 * the whole file in memory — and measures the exact size from the resulting file, which also
 * resolves the null `OpenableColumns.SIZE` case for cloud/virtual sources (the size is the number
 * of bytes actually read, not a possibly-absent metadata column).
 */
object UploadFileSource {

    private const val COPY_BUFFER_BYTES = 64 * 1024

    fun copyToCache(resolver: ContentResolver, uri: Uri, cacheDir: File): Result<CachedUpload> =
        runCatching {
            val displayName = queryDisplayName(resolver, uri) ?: fallbackName(uri)
            val contentType = resolver.getType(uri) ?: UploadDefaults.CONTENT_TYPE

            val dir = File(cacheDir, "uploads").apply { mkdirs() }
            val dest = File.createTempFile("upload_", ".bin", dir)

            val written = resolver.openInputStream(uri).use { input ->
                if (input == null) {
                    throw IOException("Cannot open content URI for reading: $uri")
                }
                dest.outputStream().use { output -> input.copyTo(output, COPY_BUFFER_BYTES) }
            }

            if (written <= 0L) {
                dest.delete()
                throw IOException("Picked file is empty or unreadable: $uri")
            }

            CachedUpload(
                file = dest,
                filename = displayName,
                contentType = contentType,
                sizeBytes = written
            )
        }

    private fun queryDisplayName(resolver: ContentResolver, uri: Uri): String? =
        resolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { cursor ->
            if (cursor.moveToFirst()) {
                val idx = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                if (idx >= 0 && !cursor.isNull(idx)) cursor.getString(idx) else null
            } else {
                null
            }
        }

    private fun fallbackName(uri: Uri): String =
        uri.lastPathSegment?.substringAfterLast('/')?.takeIf { it.isNotBlank() } ?: UploadDefaults.FILENAME
}
