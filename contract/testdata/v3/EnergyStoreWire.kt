package at.eegfaktura.integration.energystore

import com.fasterxml.jackson.annotation.JsonIgnoreProperties
import com.fasterxml.jackson.annotation.JsonProperty

// The JSON shapes of the legacy energystore (m03-energystore-reference.md §4.3). Nullable wherever
// the Go side may send null or leave a field out. All times are epoch milliseconds.

@JsonIgnoreProperties(ignoreUnknown = true)
data class EsMetaPeriod(val periodBegin: Long = 0, val periodEnd: Long = 0)

data class EsRawRequest(val meters: List<String>, val start: Long, val end: Long)

@JsonIgnoreProperties(ignoreUnknown = true)
data class EsRawRow(val ts: Long = 0, val value: List<Double>? = null, val qov: List<Int>? = null)

@JsonIgnoreProperties(ignoreUnknown = true)
data class EsRawMeter(val direction: String? = null, val data: List<EsRawRow>? = null)

@JsonIgnoreProperties(ignoreUnknown = true)
data class EsReportInterval(val type: String = "YM", val year: Int = 0, val segment: Int = 0)

@JsonIgnoreProperties(ignoreUnknown = true)
data class EsMeterReport(
    val meterId: String = "",
    val meterDir: String = "",
    val from: Long = 0,
    val until: Long = 0,
    val report: EsReport? = null,
)

@JsonIgnoreProperties(ignoreUnknown = true)
data class EsParticipantReport(val participantId: String = "", val meters: List<EsMeterReport> = emptyList())

data class EsReportRequest(val reportInterval: EsReportInterval, val participants: List<EsParticipantReport>)

@JsonIgnoreProperties(ignoreUnknown = true)
data class EsReport(val id: String? = null, val summary: EsRecord? = null, val intermediate: EsIntermediate? = null)

@JsonIgnoreProperties(ignoreUnknown = true)
data class EsRecord(val consumption: Double = 0.0, val utilization: Double = 0.0, val allocation: Double = 0.0, val production: Double = 0.0)

@JsonIgnoreProperties(ignoreUnknown = true)
data class EsIntermediate(
    val consumption: List<Double>? = null,
    val utilization: List<Double>? = null,
    val allocation: List<Double>? = null,
    val production: List<Double>? = null,
)

@JsonIgnoreProperties(ignoreUnknown = true)
data class EsCounterPointMeta(
    val name: String = "",
    val dir: String? = null,
    @param:JsonProperty("period_start") @get:JsonProperty("period_start") val periodStart: String? = null,
    @param:JsonProperty("period_end") @get:JsonProperty("period_end") val periodEnd: String? = null,
)

@JsonIgnoreProperties(ignoreUnknown = true)
data class EsReportResponse(
    val id: String? = null,
    val participantReports: List<EsParticipantReport>? = null,
    val meta: List<EsCounterPointMeta>? = null,
    val totalProduction: Double? = null,
    val totalConsumption: Double? = null,
)

data class EsSummaryRequest(val type: String, val year: Int, val segment: Int)

@JsonIgnoreProperties(ignoreUnknown = true)
data class EsReportData(
    val consumed: Double = 0.0,
    val allocated: Double = 0.0,
    val distributed: Double = 0.0,
    val produced: Double = 0.0,
    val unused: Double? = null,
    val qoVConsumer: Int? = null,
    val qoVProducer: Int? = null,
    val cntProducer: Int? = null,
    val cntConsumer: Int? = null,
)

data class EsRangeRequest(val start: Long, val end: Long)

data class EsDeleteRequest(val meteringPoint: String, val start: Long, val end: Long, val dryRun: Boolean)

@JsonIgnoreProperties(ignoreUnknown = true)
data class EsDeleteResponse(val meteringPoint: String = "", val affectedTimesteps: Int = 0, val sumKwh: Double = 0.0, val deleted: Boolean = false)
