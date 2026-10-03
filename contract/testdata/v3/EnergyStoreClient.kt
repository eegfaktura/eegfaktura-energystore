package at.eegfaktura.integration.energystore

import at.eegfaktura.integration.token.ForwardedTokenProvider
import at.eegfaktura.integration.token.NoForwardableTokenException
import at.eegfaktura.integration.token.ServiceAccountTokenProvider
import org.slf4j.LoggerFactory
import org.springframework.core.ParameterizedTypeReference
import org.springframework.http.HttpStatusCode
import org.springframework.http.MediaType
import org.springframework.http.client.JdkClientHttpRequestFactory
import java.net.http.HttpClient
import org.springframework.stereotype.Component
import org.springframework.web.client.ResourceAccessException
import org.springframework.web.client.RestClient
import org.springframework.web.client.RestClientResponseException

/**
 * The legacy energystore's REST API (m03 T2, reference §4.3): one function per endpoint, the caller's
 * own Keycloak token (or the service account's for app tokens and jobs), `X-Tenant` from the target,
 * the community id as a path variable. Failures become the four [EnergyStoreException]s.
 */
@Component
class EnergyStoreClient(
    props: EnergyStoreProperties,
    builder: RestClient.Builder,
    private val tokens: ForwardedTokenProvider,
    private val serviceAccount: ServiceAccountTokenProvider,
) {
    private val log = LoggerFactory.getLogger(EnergyStoreClient::class.java)

    private val rest =
        builder
            .baseUrl(props.baseUrl)
            // java.net.http, not HttpURLConnection: the latter drops the error body of a 401 to a POST,
            // and that body is the energystore's only explanation (e.g. a nested `tenant` claim).
            // HTTP/1.1: the energystore speaks nothing else, and an h2c upgrade on a POST is refused.
            .requestFactory(
                JdkClientHttpRequestFactory(HttpClient.newBuilder().version(HttpClient.Version.HTTP_1_1).connectTimeout(props.connectTimeout).build()).apply {
                    setReadTimeout(props.readTimeout)
                }
            )
            .build()

    /** The caller's own Keycloak token (concept 4.1); app tokens and jobs have none → the service account. */
    private fun bearer(t: EnergyStoreTarget): String =
        try {
            tokens.currentUserToken(t.tenant).also { log.debug("event=energystore.token source=user") }
        } catch (_: NoForwardableTokenException) {
            serviceAccount.token().also { log.debug("event=energystore.token source=service") }
        }

    fun meta(t: EnergyStoreTarget): Map<String, EsMetaPeriod> =
        call("meta", t, 0) { get("/eeg/v2/{ecid}/meta", t).body(object : ParameterizedTypeReference<Map<String, EsMetaPeriod>>() {}) } ?: emptyMap()

    /** `null` when the store has no data (404). */
    fun lastRecordDate(t: EnergyStoreTarget): String? =
        try {
            call("lastRecordDate", t, 0) {
                get("/eeg/{ecid}/lastRecordDate", t).body(object : ParameterizedTypeReference<Map<String, Any?>>() {})
            }?.get("periodEnd")?.toString()?.takeIf { it.isNotBlank() }
        } catch (e: EnergyStoreRefusedException) {
            if (e.status == 404) null else throw e
        }

    fun raw(t: EnergyStoreTarget, meters: List<String>, start: Long, end: Long): Map<String, EsRawMeter> {
        require(meters.isNotEmpty()) { "energystore raw: meters must not be empty" }
        return call("raw", t, meters.size) {
            // The meters in the body (v1) and as `cp` (energystore-v2, which ignores the body).
            rest.post()
                .uri { b -> b.path("/eeg/v2/{ecid}/raw").also { u -> meters.forEach { m -> u.queryParam("cp", m) } }.build(t.communityId) }
                .headers(t)
                .contentType(MediaType.APPLICATION_JSON)
                .body(EsRawRequest(meters, start, end))
                .retrieve()
                .body(object : ParameterizedTypeReference<Map<String, EsRawMeter>>() {})
        } ?: emptyMap()
    }

    fun report(t: EnergyStoreTarget, request: EsReportRequest): EsReportResponse =
        call("report", t, request.participants.sumOf { it.meters.size }) { post("/eeg/v2/{ecid}/report", t, request).body(EsReportResponse::class.java) }
            ?: EsReportResponse()

    fun summary(t: EnergyStoreTarget, request: EsSummaryRequest): EsReportData =
        call("summary", t, 0) { post("/eeg/v2/{ecid}/summary", t, request).body(object : ParameterizedTypeReference<List<EsReportData>>() {}) }
            ?.firstOrNull() ?: EsReportData()

    fun intraDay(t: EnergyStoreTarget, start: Long, end: Long): List<EsReportData> =
        call("intraDay", t, 0) { post("/eeg/v2/{ecid}/intra-day-report", t, EsRangeRequest(start, end)).body(object : ParameterizedTypeReference<List<EsReportData>>() {}) }
            .orEmpty()

    fun loadCurve(t: EnergyStoreTarget, start: Long, end: Long): List<Map<String, Any?>> =
        call("loadCurve", t, 0) {
            post("/eeg/v2/{ecid}/load-curve-report", t, EsRangeRequest(start, end)).body(object : ParameterizedTypeReference<List<Map<String, Any?>>>() {})
        }.orEmpty()

    fun deleteRaw(t: EnergyStoreTarget, request: EsDeleteRequest): EsDeleteResponse =
        call("rawdataDelete", t, 1) { post("/eeg/v2/{ecid}/rawdata/delete", t, request).body(EsDeleteResponse::class.java) }
            ?: EsDeleteResponse(request.meteringPoint)

    private fun <S : RestClient.RequestHeadersSpec<S>> S.headers(t: EnergyStoreTarget): S =
        header("Authorization", "Bearer ${bearer(t)}").header("X-Tenant", t.tenant).accept(MediaType.APPLICATION_JSON)

    private fun get(path: String, t: EnergyStoreTarget): RestClient.ResponseSpec = rest.get().uri(path, t.communityId).headers(t).retrieve()

    private fun post(path: String, t: EnergyStoreTarget, body: Any): RestClient.ResponseSpec =
        rest.post().uri(path, t.communityId).headers(t).contentType(MediaType.APPLICATION_JSON).body(body).retrieve()

    private fun <T> call(endpoint: String, t: EnergyStoreTarget, meters: Int, block: () -> T?): T? {
        val started = System.nanoTime()
        try {
            val result = block()
            log.debug("event=energystore.call endpoint={} eegId={} tenant={} meters={} status=200 ms={}", endpoint, t.eegId, t.tenant, meters, ms(started))
            return result
        } catch (e: RestClientResponseException) {
            val status: HttpStatusCode = e.statusCode
            log.warn("event=energystore.call endpoint={} eegId={} tenant={} meters={} status={} ms={}", endpoint, t.eegId, t.tenant, meters, status.value(), ms(started))
            throw when {
                status.value() == 403 -> EnergyStoreForbiddenException(endpoint)
                status.is5xxServerError -> EnergyStoreUnavailableException(endpoint, e)
                else -> EnergyStoreRefusedException(endpoint, status.value(), e.responseBodyAsString.take(500))
            }
        } catch (e: ResourceAccessException) {
            log.warn("event=energystore.call endpoint={} eegId={} tenant={} status=unreachable ms={}", endpoint, t.eegId, t.tenant, ms(started))
            throw EnergyStoreUnavailableException(endpoint, e)
        }
    }

    private fun ms(started: Long) = (System.nanoTime() - started) / 1_000_000
}
