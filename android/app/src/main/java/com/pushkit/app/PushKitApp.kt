package com.pushkit.app

import android.app.Application
import com.pushkit.app.data.upload.UploadNotifications

class PushKitApp : Application() {
    override fun onCreate() {
        super.onCreate()
        UploadNotifications.createChannel(this)
    }
}
