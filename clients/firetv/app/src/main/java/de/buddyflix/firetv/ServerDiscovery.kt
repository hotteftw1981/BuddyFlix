package de.buddyflix.firetv

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.HttpURLConnection
import java.net.Inet4Address
import java.net.InetAddress
import java.net.NetworkInterface
import java.net.SocketTimeoutException
import java.net.URL
import java.util.concurrent.Callable
import java.util.concurrent.Executors

object ServerDiscovery {
    suspend fun discover(): List<ServerCandidate> = withContext(Dispatchers.IO) {
        val found = linkedMapOf<String, ServerCandidate>()
        broadcastDiscover(found)
        if (found.isEmpty()) subnetDiscover(found)
        found.values.toList()
    }

    private fun broadcastDiscover(found: MutableMap<String, ServerCandidate>) {
        runCatching {
            DatagramSocket().use { socket ->
                socket.broadcast = true
                socket.soTimeout = 350
                val payload = "BUDDYFLIX_DISCOVER_V1".toByteArray(Charsets.UTF_8)
                socket.send(
                    DatagramPacket(
                        payload,
                        payload.size,
                        InetAddress.getByName("255.255.255.255"),
                        8097
                    )
                )

                val deadline = System.currentTimeMillis() + 900L
                while (System.currentTimeMillis() < deadline) {
                    val buffer = ByteArray(1024)
                    val response = DatagramPacket(buffer, buffer.size)
                    try {
                        socket.receive(response)
                        candidateFromDiscovery(
                            String(response.data, 0, response.length, Charsets.UTF_8),
                            response.address.hostAddress.orEmpty()
                        )?.let { found[it.id] = it }
                    } catch (_: SocketTimeoutException) {
                        // Wait until the short discovery window closes.
                    }
                }
            }
        }
    }

    private fun subnetDiscover(found: MutableMap<String, ServerCandidate>) {
        val prefixes = localPrefixes()
        if (prefixes.isEmpty()) return

        val pool = Executors.newFixedThreadPool(28)
        try {
            val tasks = mutableListOf<Callable<ServerCandidate?>>()
            for (prefix in prefixes) {
                for (host in 1..254) {
                    tasks += Callable { probe(prefix + "." + host) }
                }
            }
            for (future in pool.invokeAll(tasks)) {
                runCatching { future.get() }.getOrNull()?.let { found[it.id] = it }
            }
        } finally {
            pool.shutdownNow()
        }
    }

    private fun localPrefixes(): Set<String> {
        val prefixes = linkedSetOf<String>()
        val interfaces = runCatching { NetworkInterface.getNetworkInterfaces() }.getOrNull() ?: return prefixes
        while (interfaces.hasMoreElements()) {
            val network = interfaces.nextElement()
            if (runCatching { !network.isUp || network.isLoopback }.getOrDefault(true)) continue
            val addresses = network.inetAddresses
            while (addresses.hasMoreElements()) {
                val address = addresses.nextElement()
                if (address !is Inet4Address || !address.isSiteLocalAddress) continue
                val parts = address.hostAddress?.split(".").orEmpty()
                if (parts.size == 4) prefixes += parts.take(3).joinToString(".")
            }
        }
        return prefixes
    }

    private fun probe(host: String): ServerCandidate? {
        val connection = runCatching {
            URL("http://$host:8096/api/v1/info").openConnection() as HttpURLConnection
        }.getOrNull() ?: return null

        return try {
            connection.requestMethod = "GET"
            connection.connectTimeout = 220
            connection.readTimeout = 320
            connection.setRequestProperty("Accept", "application/json")
            if (connection.responseCode != 200) return null
            val payload = connection.inputStream.bufferedReader(Charsets.UTF_8).use { it.readText() }
            val json = JSONObject(payload)
            if (json.optString("product") != "BuddyFlix Media Server") return null
            val baseUrl = "http://$host:8096"
            ServerCandidate(
                name = json.optString("server_name", "BuddyFlix"),
                id = json.optString("server_id", baseUrl),
                baseUrl = baseUrl
            )
        } catch (_: Exception) {
            null
        } finally {
            connection.disconnect()
        }
    }

    private fun candidateFromDiscovery(payload: String, host: String): ServerCandidate? {
        if (host.isBlank()) return null
        return runCatching {
            val json = JSONObject(payload)
            if (json.optString("product") != "BuddyFlix Media Server") return null
            val port = json.optInt("port", 8096)
            val url = "http://$host:$port"
            ServerCandidate(
                name = json.optString("server_name", "BuddyFlix"),
                id = json.optString("server_id", url),
                baseUrl = url
            )
        }.getOrNull()
    }
}
