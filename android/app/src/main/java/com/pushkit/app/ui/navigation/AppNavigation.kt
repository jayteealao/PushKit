package com.pushkit.app.ui.navigation

import android.app.DownloadManager
import android.content.Context
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalContext
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.pushkit.app.data.CredentialStore
import com.pushkit.app.data.FileRepository
import com.pushkit.app.data.api.RetrofitProvider
import com.pushkit.app.ui.files.FileListScreen
import com.pushkit.app.ui.files.FileListViewModel
import com.pushkit.app.ui.settings.SettingsScreen
import com.pushkit.app.ui.settings.SettingsViewModel

object Routes {
    const val SETTINGS = "settings"
    const val FILE_LIST = "file_list"
}

@Composable
fun AppNavigation() {
    val context = LocalContext.current
    val credentialStore = remember { CredentialStore(context) }
    val navController = rememberNavController()

    val startDestination = if (credentialStore.isConfigured) Routes.FILE_LIST else Routes.SETTINGS

    NavHost(navController = navController, startDestination = startDestination) {
        composable(Routes.SETTINGS) {
            val viewModel = remember { SettingsViewModel(credentialStore) }
            SettingsScreen(
                viewModel = viewModel,
                onSaved = {
                    navController.navigate(Routes.FILE_LIST) {
                        popUpTo(Routes.SETTINGS) { inclusive = true }
                    }
                }
            )
        }

        composable(Routes.FILE_LIST) {
            val api = remember { RetrofitProvider.create(credentialStore) }
            val repository = remember { FileRepository(api) }
            val downloadManager = remember {
                context.getSystemService(Context.DOWNLOAD_SERVICE) as DownloadManager
            }
            val viewModel = remember { FileListViewModel(repository, downloadManager) }

            FileListScreen(
                viewModel = viewModel,
                onNavigateToSettings = {
                    navController.navigate(Routes.SETTINGS)
                }
            )
        }
    }
}
