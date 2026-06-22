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
import androidx.work.WorkManager
import com.pushkit.app.data.upload.UploadFileSource
import com.pushkit.app.data.upload.UploadWorker
import com.pushkit.app.ui.navigation.AppNavigation
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/**
 * Kept for backward-compat: the tag constant now lives in [UploadWorker.Companion] so the worker
 * and all enqueue sites share a single definition.
 */
internal val TAG_SHARE_UPLOAD get() = UploadWorker.TAG_SHARE_UPLOAD

/**
 * Extracts the share URI from an ACTION_SEND intent, copies the file to app-private cache on a
 * background thread, and enqueues a foreground upload worker — the same worker path the in-app
 * picker uses. Returns immediately; the copy and enqueue happen synchronously on the caller's thread
 * (tests call this directly on the test thread).
 *
 * Extracted as a package-level function so it can be called from [MainActivity] (which wraps the
 * call in a lifecycleScope.launch(Dispatchers.IO)) and tested directly without spinning up a full
 * Activity.
 */
internal fun enqueueFromShareIntent(
    intent: Intent?,
    context: Context
) {
    if (intent?.action != Intent.ACTION_SEND) return
    val uri: Uri = IntentCompat.getParcelableExtra(intent, Intent.EXTRA_STREAM, Uri::class.java)
        ?: return

    val cached = UploadFileSource.copyToCache(context.contentResolver, uri, context.cacheDir)
        .getOrElse { return }

    val request = UploadWorker.buildRequest(cached, tag = UploadWorker.TAG_SHARE_UPLOAD)
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
     * the WorkManager enqueue happens back on the same thread. If the Activity is destroyed
     * mid-copy the coroutine is cancelled (and the URI grant has also been revoked, so no data
     * is lost).
     */
    private fun handleShareIntent(intent: Intent?) {
        if (intent?.action != Intent.ACTION_SEND) return
        val appContext = applicationContext
        lifecycleScope.launch {
            withContext(Dispatchers.IO) {
                enqueueFromShareIntent(intent, appContext)
            }
        }
    }
}
