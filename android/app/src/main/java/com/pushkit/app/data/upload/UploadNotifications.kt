package com.pushkit.app.data.upload

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import androidx.core.app.NotificationCompat

/**
 * Notification channel + builders for the background upload worker: an ongoing progress
 * notification (used as the foreground-service notification) and a terminal success/failure
 * notification. minSdk 26, so the notification channel always exists.
 */
object UploadNotifications {

    const val CHANNEL_ID = "uploads"
    const val PROGRESS_NOTIFICATION_ID = 4101
    const val RESULT_NOTIFICATION_ID = 4102

    fun createChannel(context: Context) {
        val manager = context.getSystemService(NotificationManager::class.java) ?: return
        val channel = NotificationChannel(
            CHANNEL_ID,
            "File uploads",
            NotificationManager.IMPORTANCE_LOW
        ).apply { description = "Progress and results for background file uploads" }
        manager.createNotificationChannel(channel)
    }

    fun progress(context: Context, filename: String): Notification =
        NotificationCompat.Builder(context, CHANNEL_ID)
            .setContentTitle("Uploading")
            .setContentText(filename)
            .setSmallIcon(android.R.drawable.stat_sys_upload)
            .setOngoing(true)
            .setProgress(0, 0, true)
            .build()

    fun terminal(context: Context, title: String, text: String): Notification =
        NotificationCompat.Builder(context, CHANNEL_ID)
            .setContentTitle(title)
            .setContentText(text)
            .setSmallIcon(android.R.drawable.stat_sys_upload_done)
            .setOngoing(false)
            .setAutoCancel(true)
            .build()
}
