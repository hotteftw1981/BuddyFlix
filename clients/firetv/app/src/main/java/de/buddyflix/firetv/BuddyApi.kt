package de.buddyflix.firetv

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL

class BuddyApi(
    baseUrl: String,
    var token: String = "",
    var profileId: Long = 0L
) {
    val baseUrl: String = normalizeBaseUrl(baseUrl)

    private suspend fun request(
        method: String,
        path: String,
        body: JSONObject? = null,
        authenticated: Boolean = true
    ): String = withContext(Dispatchers.IO) {
        val connection = (URL(baseUrl + path).openConnection() as HttpURLConnection).apply {
            requestMethod = method
            connectTimeout = 6_000
            readTimeout = 20_000
            setRequestProperty("Accept", "application/json")
            setRequestProperty("Content-Type", "application/json")
            if (authenticated && token.isNotBlank()) {
                setRequestProperty("Authorization", "Bearer " + token)
            }
            if (authenticated && profileId > 0) {
                setRequestProperty("X-BuddyFlix-Profile", profileId.toString())
            }
            if (body != null) {
                doOutput = true
                outputStream.bufferedWriter(Charsets.UTF_8).use { it.write(body.toString()) }
            }
        }

        try {
            val code = connection.responseCode
            val stream = if (code in 200..299) connection.inputStream else connection.errorStream
            val payload = stream?.bufferedReader(Charsets.UTF_8)?.use { it.readText() }.orEmpty()
            if (code !in 200..299) {
                val message = runCatching { JSONObject(payload).optString("error") }.getOrDefault("")
                throw BuddyApiException(code, message.ifBlank { "HTTP " + code })
            }
            payload
        } finally {
            connection.disconnect()
        }
    }

    suspend fun serverInfo(): JSONObject {
        val raw = request("GET", "/api/v1/info", authenticated = false)
        return JSONObject(raw)
    }

    suspend fun deviceLogin(username: String, password: String, deviceName: String): DeviceLogin {
        val body = JSONObject()
            .put("username", username)
            .put("password", password)
            .put("device_name", deviceName)
        val raw = request("POST", "/api/v1/device/login", body, authenticated = false)
        val json = JSONObject(raw)
        token = json.getString("token")
        return DeviceLogin(
            token = token,
            serverName = json.optString("server_name", "BuddyFlix"),
            serverId = json.optString("server_id")
        )
    }

    suspend fun profiles(): List<BuddyProfile> {
        val json = JSONObject(request("GET", "/api/profiles"))
        val array = json.optJSONArray("profiles") ?: JSONArray()
        return buildList {
            for (i in 0 until array.length()) {
                val p = array.getJSONObject(i)
                add(BuddyProfile(
                    id = p.getLong("id"),
                    name = p.optString("name", "Profil"),
                    avatar = p.optString("avatar", "🎬")
                ))
            }
        }
    }

    suspend fun home(): HomeData {
        val media = parseMediaArray(JSONArray(request("GET", "/api/media")))
        val seriesJson = JSONArray(request("GET", "/api/series"))
        val series = buildList {
            for (i in 0 until seriesJson.length()) {
                val s = seriesJson.getJSONObject(i)
                val next = s.optJSONObject("next_episode")?.let(::parseMedia)
                add(SeriesEntry(
                    key = s.optString("key"),
                    title = s.optString("title", "Serie"),
                    seasonCount = s.optInt("season_count"),
                    episodeCount = s.optInt("episode_count"),
                    watchedCount = s.optInt("watched_count"),
                    nextEpisode = next
                ))
            }
        }
        return HomeData(media, series)
    }

    suspend fun saveProgress(mediaId: Long, positionSeconds: Double, durationSeconds: Double) {
        if (durationSeconds <= 0.0) return
        val body = JSONObject()
            .put("media_id", mediaId)
            .put("Position", positionSeconds)
            .put("Duration", durationSeconds)
        request("POST", "/api/progress", body)
    }

    suspend fun logoutDevice() {
        request("POST", "/api/v1/device/logout", JSONObject())
    }

    fun streamUrl(mediaId: Long): String = baseUrl + "/stream/" + mediaId

    fun streamHeaders(): Map<String, String> = buildMap {
        if (token.isNotBlank()) put("Authorization", "Bearer " + token)
        if (profileId > 0) put("X-BuddyFlix-Profile", profileId.toString())
    }

    private fun parseMediaArray(array: JSONArray): List<MediaEntry> = buildList {
        for (i in 0 until array.length()) add(parseMedia(array.getJSONObject(i)))
    }

    private fun parseMedia(m: JSONObject) = MediaEntry(
        id = m.getLong("id"),
        title = m.optString("title", "Unbekannt"),
        year = m.optInt("year"),
        overview = m.optString("overview"),
        progress = m.optDouble("progress", 0.0),
        position = m.optDouble("position", 0.0),
        duration = m.optDouble("duration", 0.0),
        progressUpdated = m.optString("progress_updated"),
        kind = m.optString("kind", "movie"),
        seriesTitle = m.optString("series_title"),
        season = m.optInt("season"),
        episode = m.optInt("episode")
    )

    companion object {
        fun normalizeBaseUrl(input: String): String {
            var value = input.trim().trimEnd('/')
            if (value.isBlank()) return ""
            if (!value.startsWith("http://") && !value.startsWith("https://")) {
                value = "http://" + value
            }
            return value
        }
    }
}

class BuddyApiException(val status: Int, message: String) : Exception(message)
