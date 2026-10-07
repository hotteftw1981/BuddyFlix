package de.buddyflix.firetv

data class DeviceLogin(
    val token: String,
    val serverName: String,
    val serverId: String
)

data class BuddyProfile(
    val id: Long,
    val name: String,
    val avatar: String
)

data class MediaEntry(
    val id: Long,
    val title: String,
    val year: Int,
    val overview: String,
    val progress: Double,
    val position: Double,
    val duration: Double,
    val progressUpdated: String,
    val kind: String,
    val seriesTitle: String,
    val season: Int,
    val episode: Int
) {
    val subtitle: String
        get() = if (kind == "episode") {
            val code = "S" + season.toString().padStart(2, '0') +
                "E" + episode.toString().padStart(2, '0')
            listOf(seriesTitle, code).filter { it.isNotBlank() }.joinToString(" · ")
        } else {
            if (year > 0) year.toString() else "Film"
        }
}

data class SeriesEntry(
    val key: String,
    val title: String,
    val seasonCount: Int,
    val episodeCount: Int,
    val watchedCount: Int,
    val nextEpisode: MediaEntry?
)

data class HomeData(
    val media: List<MediaEntry>,
    val series: List<SeriesEntry>
)
