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
    val poster: String,
    val backdrop: String,
    val runtime: Int,
    val added: String,
    val progress: Double,
    val position: Double,
    val duration: Double,
    val progressUpdated: String,
    val favorite: Boolean,
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
            buildList {
                if (year > 0) add(year.toString())
                if (runtime > 0) add(runtime.toString() + " Min.")
            }.joinToString(" · ").ifBlank { "Film" }
        }
}

data class SeriesSeason(
    val number: Int,
    val episodes: List<MediaEntry>
)

data class SeriesEntry(
    val key: String,
    val title: String,
    val seasonCount: Int,
    val episodeCount: Int,
    val watchedCount: Int,
    val continueCount: Int,
    val poster: String,
    val backdrop: String,
    val lastActivity: String,
    val nextEpisode: MediaEntry?,
    val seasons: List<SeriesSeason>
)

data class HomeData(
    val media: List<MediaEntry>,
    val series: List<SeriesEntry>
)

data class ServerCandidate(
    val name: String,
    val id: String,
    val baseUrl: String
)
