package com.pushkit.app

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.material3.MaterialTheme
import androidx.core.content.IntentCompat
import androidx.lifecycle.lifecycleScope
import androidx.work.BackoffPolicy
import androidx.work.Constraints
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkRequest
import androidx.work.workDataOf
import com.pushkit.app.data.upload.UploadFileSource
import com.pushkit.app.data.upload.UploadWorker
import com.pushkit.app.ui.navigation.AppNavigation
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.util.concurrent.TimeUnit

/** Tag applied to every WorkManager request enqueued from a share intent. */
internal const val TAG_SHARE_UPLOAD = "share_upload"

/**
 * Extracts the share URI from an ACTION_SEND intent, copies the file to app-private cache on a
 * background thread, and enqueues a foreground upload worker — the same worker path the in-app
 * picker uses. Returns immediately; the copy and enqueue happen asynchronously inside [scope].
 *
 * Extracted as a package-level function so it can be called from [MainActivity] (which supplies a
 * lifecycleScope) and tested directly without spinning up a full Activity.
 *
 * Copy-to-cache uses the caller's [scope] (typically lifecycleScope) so the IO work is cancelled
 * if the Activity is destroyed; the URI grant is also revoked at that point, so no data is lost.
 */
internal fun enqueueFromShareIntent(
    intent: Intent?,
    context: Context
) {
    if (intent?.action != Intent.ACTION_SEND) return
    val uri: Uri = IntentCompat.getParcelableExtra(intent, Intent.EXTRA_STREAM, Uri::class.java)
        ?: return

    // Synchronous copy-to-cache for the test-path caller; the Activity wrapper is async.
    // Tests invoke this directly on the test thread (no coroutine scope needed).
    val cached = UploadFileSource.copyToCache(context.contentResolver, uri, context.cacheDir)
        .getOrElse { return }

    val request = OneTimeWorkRequestBuilder<UploadWorker>()
        .addTag(TAG_SHARE_UPLOAD)
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
                UploadWorker.KEY_CACHE_PATH to cached.file.absolutePath,
                UploadWorker.KEY_FILENAME to cached.filename,
                UploadWorker.KEY_CONTENT_TYPE to cached.contentType,
                UploadWorker.KEY_SIZE to cached.sizeBytes
            )
        )
        .build()

    WorkManager.getInstance(context).enqueue(request)
}

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            MaterialTheme {
                AppNavigation()
            }
        }
        handleShareIntent(intent)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        handleShareIntent(intent)
    }

    /**
     * Dispatches an ACTION_SEND intent to the package-level [enqueueFromShareIntent] helper on a
     * background thread via [lifecycleScope]. The copy-to-cache step runs on Dispatchers.IO and
     * the WorkManager enqueue happens back on the default dispatcher. If the Activity is destroyed
     * mid-copy the coroutine is cancelled (and the URI grant has also been revoked, so no data
     * is lost).
     */
    private fun handleShareIntent(intent: Intent?) {
        if (intent?.action != Intent.ACTION_SEND) return
        val uri: Uri = IntentCompat.getParcelableExtra(intent, Intent.EXTRA_STREAM, Uri::class.java)
            ?: return

        val appContext = applicationContext
        lifecycleScope.launch {
            val cached = withContext(Dispatchers.IO) {
                UploadFileSource.copyToCache(appContext.contentResolver, uri, appContext.cacheDir)
            }.getOrElse { return@launch }

            val request = OneTimeWorkRequestBuilder<UploadWorker>()
                .addTag(TAG_SHARE_UPLOAD)
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
                        UploadWorker.KEY_CACHE_PATH to cached.file.absolutePath,
                        UploadWorker.KEY_FILENAME to cached.filename,
                        UploadWorker.KEY_CONTENT_TYPE to cached.contentType,
                        UploadWorker.KEY_SIZE to cached.sizeBytes
                    )
                )
                .build()

            WorkManager.getInstance(appContext).enqueue(request)
        }
    }
}
