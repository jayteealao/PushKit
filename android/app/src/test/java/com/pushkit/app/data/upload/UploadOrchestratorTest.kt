package com.pushkit.app.data.upload

import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import com.pushkit.app.data.FileRepository
import com.pushkit.app.data.api.PushKitApi
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import retrofit2.Retrofit
import java.io.File

/**
 * Pure-JVM test of the init -> presigned PUT -> complete orchestration (no device / Robolectric).
 * MockWebServer stands in for both the PushKit API and the S3 presigned URL.
 */
class UploadOrchestratorTest {

    private lateinit var server: MockWebServer
    private lateinit var orchestrator: UploadOrchestrator
    private lateinit var tempFile: File

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()

        val json = Json { ignoreUnknownKeys = true }
        val api = Retrofit.Builder()
            .baseUrl(server.url("/"))
            .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(PushKitApi::class.java)
        orchestrator = UploadOrchestrator(FileRepository(api), OkHttpClient())

        tempFile = File.createTempFile("upload-test", ".txt")
        tempFile.writeText("hello pushkit upload")
    }

    @After
    fun tearDown() {
        server.shutdown()
        tempFile.delete()
    }

    // contentType here ("text/markdown") is what the client sends to init; the server echoes back a
    // DIFFERENT signed Content-Type ("text/plain") to prove the PUT replays the server's value.
    private fun cached() = CachedUpload(
        file = tempFile,
        filename = "note.txt",
        contentType = "text/markdown",
        sizeBytes = tempFile.length()
    )

    @Test
    fun upload_replaysSignedContentType_setsExactLength_andCompletes() = runTest {
        server.enqueue(
            MockResponse().setBody(
                """{"fileId":"f1","s3Key":"k","presignedPutUrl":"${server.url("/put")}","expiresInSeconds":900,"requiredHeaders":{"Content-Type":"text/plain"}}"""
            )
        )
        server.enqueue(MockResponse().setResponseCode(200)) // S3 PUT
        server.enqueue(
            MockResponse().setBody(
                """{"id":"f1","originalFilename":"note.txt","contentType":"text/plain","sizeBytes":${tempFile.length()},"createdAt":"2026-06-22T00:00:00Z","status":"UPLOADED"}"""
            )
        )

        val result = orchestrator.upload(cached())
        assertTrue(result.isSuccess)
        assertEquals("f1", result.getOrThrow().id)

        val init = server.takeRequest()
        assertEquals("POST", init.method)
        assertEquals("/v1/uploads/init", init.path)
        assertTrue(init.body.readUtf8().contains("\"contentType\":\"text/markdown\""))

        val put = server.takeRequest()
        assertEquals("PUT", put.method)
        assertEquals("/put", put.path)
        assertEquals("text/plain", put.getHeader("Content-Type")) // replayed signed value
        assertEquals(tempFile.length(), put.getHeader("Content-Length")?.toLong())
        assertNull(put.getHeader("Transfer-Encoding")) // exact length => not chunked

        val complete = server.takeRequest()
        assertEquals("POST", complete.method)
        assertEquals("/v1/uploads/complete", complete.path)
    }

    @Test
    fun upload_putFails_cleansUpOrphan_andReportsFailure() = runTest {
        server.enqueue(
            MockResponse().setBody(
                """{"fileId":"f9","s3Key":"k","presignedPutUrl":"${server.url("/put")}","expiresInSeconds":900,"requiredHeaders":{"Content-Type":"text/plain"}}"""
            )
        )
        server.enqueue(MockResponse().setResponseCode(500)) // S3 PUT fails
        server.enqueue(MockResponse().setResponseCode(204)) // DELETE orphan

        val result = orchestrator.upload(cached())
        assertFalse(result.isSuccess)

        assertEquals("/v1/uploads/init", server.takeRequest().path)
        assertEquals("/put", server.takeRequest().path)
        val delete = server.takeRequest()
        assertEquals("DELETE", delete.method)
        assertEquals("/v1/files/f9", delete.path)
        // complete must NOT have been called.
        assertEquals(3, server.requestCount)
    }

    /**
     * TST-3: init+PUT succeed but complete() returns an error.
     *
     * The orchestrator must:
     *  1. Issue the best-effort orphan DELETE (the file exists in S3 but is INITIATED in the DB).
     *  2. Return a failure result — the caller (UploadWorker) can decide whether to retry.
     *
     * Exactly 4 requests must be observed: init, PUT, complete, DELETE (orphan cleanup).
     */
    @Test
    fun upload_completeFails_cleansUpOrphan_andReportsFailure() = runTest {
        server.enqueue(
            MockResponse().setBody(
                """{"fileId":"f2","s3Key":"k2","presignedPutUrl":"${server.url("/put")}","expiresInSeconds":900,"requiredHeaders":{"Content-Type":"text/plain"}}"""
            )
        )
        server.enqueue(MockResponse().setResponseCode(200)) // S3 PUT succeeds
        server.enqueue(MockResponse().setResponseCode(500)) // complete() fails
        server.enqueue(MockResponse().setResponseCode(204)) // DELETE orphan (best-effort)

        val result = orchestrator.upload(cached())
        assertFalse("complete() failure must propagate as a failure result", result.isSuccess)

        assertEquals("/v1/uploads/init", server.takeRequest().path)
        assertEquals("/put", server.takeRequest().path)
        assertEquals("/v1/uploads/complete", server.takeRequest().path)
        val delete = server.takeRequest()
        assertEquals("DELETE", delete.method)
        assertEquals("/v1/files/f2", delete.path)
        assertEquals("complete+orphan-delete: exactly 4 requests expected", 4, server.requestCount)
    }
}
