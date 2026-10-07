package de.buddyflix.firetv

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.InetAddress
import java.net.SocketTimeoutException

object ServerDiscovery {
    suspend fun discover(): List<ServerCandidate> = withContext(Dispatchers.IO) {
        val found = linkedMapOf<String, ServerCandidate>()
        DatagramSocket().use { socket ->
            socket.broadcast = true
            socket.soTimeout = 650
            val payload = "BUDDYFLIX_DISCOVER_V1".toByteArray(Charsets.UTF_8)
            val packet = DatagramPacket(
                payload,
                payload.size,
                InetAddress.getByName("255.255.255.255"),
                8097
            )
            runCatching { socket.send(packet) }

            val deadline = System.currentTimeMillis() + 1800L
            while (System.currentTimeMillis() < deadline) {
                val buffer = ByteArray(1024)
                val response = DatagramPacket(buffer, buffer.size)
                try {
                    socket.receive(response)
                    val json = JSONObject(String(response.data, 0, response.length, Charsets.UTF_8))
                    if (json.optString("product") != "BuddyFlix Media Server") continue
                    val port = json.optInt("port", 8096)
                    val host = response.address.hostAddress ?: continue
                    val url = "http://$host:$port"
                    val candidate = ServerCandidate(
                        name = json.optString("server_name", "BuddyFlix"),
                        id = json.optString("server_id", url),
                        baseUrl = url
                    )
                    found[candidate.id] = candidate
                } catch (_: SocketTimeoutException) {
                    // Keep looping until the discovery window closes.
                } catch (_: Exception) {
                    // Discovery is best-effort; manual entry remains available.
                }
            }
        }
        found.values.toList()
    }
}
