package couchbase

import (
	"crypto/tls"
	"encoding/json"
	"exporter/exporter/utility"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

var logger = utility.Logger()

var X509KeyPair tls.Certificate

const EMX_THROTTLE_TIME = 25

var CB_CONNECTIONSTRING = ""

// Throttling timer
var lastCallTime time.Time

// Bucket struct for json response unmarshalling
type bucketMetric struct {
	bucket_name                string
	bucket_replica_count       int
	bucket_eviction_type       string
	bucket_compression_type    string
	bucket_storage_backend     string
	bucket_conflict_resolution string
}

// Index struct for json response unmarshalling
type indexMetric struct {
	index_name          string
	index_replica_count int
	bucket              string
	collection          string
	scope               string
	index_type          string
}

// Convenience struct for storing all required values for metrics and labels
type response struct {
	cluster_uuid                 string
	buckets                      map[int]bucketMetric
	indexes                      map[int]indexMetric
	cluster_balanced             bool
	index_storage_engine         string
	slow_queries_threshold       int
	slow_queries_limit           int
	data_memory_quota            int
	index_memory_quota           int
	ram_quota_used               int
	autofailover_enabled         bool
	autofailover_on_disk_enabled bool
	autofailover_on_disk_timeout int
	autofailover_timeout         int
	autofailover_max_count       int
	autofailover_current_count   int
	failover_counter             int
	failover_start_counter       int
	failover_complete_counter    int
	failover_success_counter     int
	failover_stop_counter        int
	failover_fail_counter        int
	rebalance_start_counter      int
	rebalance_success_counter    int
	rebalance_fail_counter       int
	rebalance_stop_counter       int
	rebalance_status             map[string]float64
	nodes                        map[string][]string
	largest_server_group_count   int
	sever_group_count            int
}

// Metrics Collector Structure
type MetricsCollector struct {
	bucket_replica_count         *prometheus.Desc
	bucket_eviction_type         *prometheus.Desc
	bucket_compression_type      *prometheus.Desc
	bucket_storage_backend       *prometheus.Desc
	bucket_conflict_resolution   *prometheus.Desc
	index_replica_count          *prometheus.Desc
	cluster_balanced             *prometheus.Desc
	index_storage_engine         *prometheus.Desc
	slow_queries_threshold       *prometheus.Desc
	slow_queries_limit           *prometheus.Desc
	data_memory_quota            *prometheus.Desc
	index_memory_quota           *prometheus.Desc
	ram_quota_used               *prometheus.Desc
	autofailover_enabled         *prometheus.Desc
	autofailover_on_disk_enabled *prometheus.Desc
	autofailover_on_disk_timeout *prometheus.Desc
	autofailover_timeout         *prometheus.Desc
	autofailover_max_count       *prometheus.Desc
	autofailover_current_count   *prometheus.Desc
	failover_counter             *prometheus.Desc
	failover_start_counter       *prometheus.Desc
	failover_complete_counter    *prometheus.Desc
	failover_success_counter     *prometheus.Desc
	failover_stop_counter        *prometheus.Desc
	failover_fail_counter        *prometheus.Desc
	rebalance_start_counter      *prometheus.Desc
	rebalance_success_counter    *prometheus.Desc
	rebalance_fail_counter       *prometheus.Desc
	rebalance_stop_counter       *prometheus.Desc
	rebalance_status             *prometheus.Desc
	largest_server_group_count   *prometheus.Desc
	server_group_count           *prometheus.Desc
}

// Couchbase endpoints for stat gathering
const CBEMXENDPOINT_BucketStats string = "/pools/default/buckets"
const CBEMXENDPOINT_IndexStatus string = "/indexStatus"
const CBEMXENDPOINT_ClusterStatus string = "/pools/nodes"
const CBEMXENDPOINT_QuesrySettings string = "/settings/querySettings"
const CBEMXENDPOINT_IndexSettings string = "/settings/indexes"
const CBEMXENDPOINT_AutoFailover string = "/settings/autoFailover"
const CBEMXENDPOINT_Rebalance string = "/pools/default/rebalanceProgress"
const CBEMXENDPOINT_ClusterUUID string = "/pools"
const CBEMXENDPOINT_ServerGroups string = "/pools/default/serverGroups"

// CBEMXENDPOINT_BucketStats
type cbemxBucketStatsArray struct {
	Buckets []cbemxBucketStatsDetails `json:"-"`
}

// Per bucket from CBEMXENDPOINT_BucketStats
type cbemxBucketStatsDetails struct {
	EvictionPolicy         string `json:"evictionPolicy"`
	BucketName             string `json:"name"`
	ConflictResolutionType string `json:"conflictResolutionType"`
	StorageBackend         string `json:"storageBackend"`
	CompressionMode        string `json:"compressionMode"`
	VBucketServerMap       struct {
		NumReplicas int `json:"numReplicas"`
	} `json:"vBucketServerMap"`
}

// /CBEMXENDPOINT_IndexStatus
type cbemxIndexStatusArray struct {
	Indexes []cbemxIndexStatusDetails `json:"indexes"`
}

// per index from CBEMXENDPOINT_IndexStatus
type cbemxIndexStatusDetails struct {
	NumReplicas int    `json:"numReplica"`
	Definition  string `json:"definition"`
	IndexName   string `json:"indexName"`
	Bucket      string `json:"bucket"`
	Collection  string `json:"collection"`
	Scope       string `json:"scope"`
	ReplicaId   int    `json:"replicaId"`
}

// CBEMXENDPOINT_ClusterStatus
type cbemxClusterStatusDetails struct {
	Nodes            []cbemxNodeDetails `json:"nodes"`
	Balanced         bool               `json:"balanced"`
	MemoryQuota      int                `json:"memoryQuota"`
	IndexMemoryQuota int                `json:"indexMemoryQuota"`
	StorageTotals    struct {
		Ram struct {
			QuotaUsed int `json:"quotaUsed"`
		} `json:"ram"`
	} `json:"storageTotals"`
	Counters struct {
		Failover          int `json:"failover"`
		Failover_start    int `json:"failover_start"`
		Failover_complete int `json:"failover_complete"`
		Failover_success  int `json:"failover_success"`
		Failover_stop     int `json:"failover_stop"`
		Failover_fail     int `json:"failover_fail"`
		Rebalance_start   int `json:"rebalance_start"`
		Rebalance_success int `json:"rebalance_success"`
		Rebalance_fail    int `json:"rebalance_fail"`
		Rebalance_stop    int `json:"rebalance_stop"`
	}
	//Nodes []map[string]interface{} `json:"nodes"` // strictly for counting number of nodes for alerting
}

// per node from CBEMXENDPOINT_ClusterStatus
type cbemxNodeDetails struct {
	Hostname string   `json:"hostname"`
	Services []string `json:"services"`
}

// CBEMXENDPOINT_ServerGroups
type cbemxServerGroupDetails struct {
	Nodes []map[string]interface{} `json:"nodes"` // strictly for counting number of nodes for alerting
}
type cbemxServerGroupsArray struct {
	Groups []cbemxServerGroupDetails `json:"groups"` // strictly for counting number of nodes for alerting
}

// CBEMXENDPOINT_QuesrySettings
type cbemxQuerySettingsDetails struct {
	Slow_queries_threshold int `json:"queryCompletedThreshold"`
	Slow_queries_limit     int `json:"queryCompletedLimit"`
}

// CBEMXENDPOINT_IndexSettings
type cbemxIndexSettingsDetails struct {
	StorageMode string `json:"storageMode"`
}

// CBEMXENDPOINT_AutoFailover
type cbemxAutoFailoverDetails struct {
	Enabled                  bool `json:"enabled"`
	Timeout                  int  `json:"timeout"`
	MaxCount                 int  `json:"maxCount"`
	Count                    int  `json:"count"`
	FailoverOnDataDiskIssues struct {
		Enabled    bool `json:"enabled"`
		TimePeriod int  `json:"timePeriod"`
	} `json:"failoverOnDataDiskIssues"`
}

/**
** CBEMXENDPOINT_Rebalance
**
** Rebalance Status json response structure:
** {
**  "status": "running",
**  "<hostname>": {
**    "progress": <float>
** },
**  "<hostname>": {
**    "progress": <float>
**  }
**}
**/
type cbemxRebalanceDetails struct {
	ProgressDetails map[string]json.RawMessage `json:"-"`
}

// CBEMXENDPOINT_ClusterUUID
type cbemxClusterUUIDDetails struct {
	UUID string `json:"uuid"`
}

/*
* Set base API url based of given Hostname.
* Defaults to 'localhost' using HTTPS on port 18091.
 */
func setCBConnectionString() {
	level.Info(logger).Log("Event", "Setting connection string based on protocol, host and port env variables")
	cbProtocol := strings.ToUpper(os.Getenv("CB_PROTOCOL"))
	if cbProtocol != "HTTP" && cbProtocol != "HTTPS" {
		cbProtocol = "HTTPS"
	}
	cbPort := os.Getenv("CB_PORT")
	if cbPort == "" {
		if cbProtocol == "HTTPS" {
			cbPort = "18091"
		} else {
			cbPort = "8091"
		}
	}
	cbHost := os.Getenv("CB_HOST")
	if cbHost == "" {
		cbHost = "localhost"
	}
	CB_CONNECTIONSTRING = cbProtocol + "://" + cbHost + ":" + cbPort
}

/*
* Generic method for populating metrics structs from endpoint responses.
* param: cbHostUrl {string} - base url for API call to server
* param: apiEndpoint {string} - CBEMX endpoint to call
* param: cbemxStruct {interface{}} - pointer to the relevant structure for json unmarshalling of the endpoint response
 */
func getCbemxForApi(apiEndpoint string, cbemxStruct interface{}) {

	cbStatsApi := CB_CONNECTIONSTRING + apiEndpoint

	// Fetching the cb bucket stats details using api
	// http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{Certificates: []tls.Certificate{X509KeyPair}, InsecureSkipVerify: true}
	cbStatsDetails, err := http.Get(cbStatsApi)
	if err != nil {
		level.Error(logger).Log("Error", err)
	}

	// Closing the response body and terminating the connection
	defer cbStatsDetails.Body.Close()

	// check response code
	if cbStatsDetails.StatusCode == http.StatusOK {
		level.Debug(logger).Log("Debug", "Request to "+apiEndpoint+" was successful. Status code="+strconv.Itoa(cbStatsDetails.StatusCode))
	} else if cbStatsDetails.StatusCode == http.StatusUnauthorized {
		level.Error(logger).Log("Error", "Unauthorized access from "+apiEndpoint+". Status code="+strconv.Itoa(cbStatsDetails.StatusCode))
		// Handle unauthorized access here
	} else if cbStatsDetails.StatusCode == http.StatusForbidden {
		level.Error(logger).Log("Error", "Access forbidden (403) from "+apiEndpoint+". Status code="+strconv.Itoa(cbStatsDetails.StatusCode))
		// Handle forbidden access here
	} else {
		level.Error(logger).Log("Error", "Unexpected status code when calling "+apiEndpoint+". Status code="+strconv.Itoa(cbStatsDetails.StatusCode))
	}

	// Converting the  details http response to json body
	cbemxDetailsBytes, err := ioutil.ReadAll(cbStatsDetails.Body)

	if err != nil {
		level.Error(logger).Log("Error", err)
	}
	json.Unmarshal(cbemxDetailsBytes, cbemxStruct)

}

/*
Get stats for each API endpoint and populate the response struct with collated values
*/
func getCbemxStats() (exported response) {
	var (
		cbemxBucketStatsStructArray cbemxBucketStatsArray
		cbemxIndexStatusStructArray cbemxIndexStatusArray
		cbemxClusterStatusStruct    cbemxClusterStatusDetails
		cbemxIndexSettingsStruct    cbemxIndexSettingsDetails
		cbemxAutoFailoverStruct     cbemxAutoFailoverDetails
		cbemxRebalanceStruct        cbemxRebalanceDetails
		cbemxClusterUUIDStruct      cbemxClusterUUIDDetails
		cbemxQuerySettingsStruct    cbemxQuerySettingsDetails
		cbemxServerGroupStruct      cbemxServerGroupsArray
	)

	//var exported response
	level.Info(logger).Log("Event", "Collecting stats from CB endpoints.")
	level.Info(logger).Log("Event", "Collecting stats from CB cbemxBucketStatsStructArray.")
	getCbemxForApi(CBEMXENDPOINT_BucketStats, &cbemxBucketStatsStructArray.Buckets)
	level.Info(logger).Log("Event", "Collecting stats from CB cbemxIndexStatusStructArray.")
	getCbemxForApi(CBEMXENDPOINT_IndexStatus, &cbemxIndexStatusStructArray)
	level.Info(logger).Log("Event", "Collecting stats from CB cbemxClusterStatusStruct.")
	getCbemxForApi(CBEMXENDPOINT_ClusterStatus, &cbemxClusterStatusStruct)
	level.Info(logger).Log("Event", "Collecting stats from CB cbemxQuerySettingsStruct.")
	getCbemxForApi(CBEMXENDPOINT_QuesrySettings, &cbemxQuerySettingsStruct)
	level.Info(logger).Log("Event", "Collecting stats from CB cbemxIndexSettingsStruct.")
	getCbemxForApi(CBEMXENDPOINT_IndexSettings, &cbemxIndexSettingsStruct)
	level.Info(logger).Log("Event", "Collecting stats from CB cbemxAutoFailoverStruct.")
	getCbemxForApi(CBEMXENDPOINT_AutoFailover, &cbemxAutoFailoverStruct)
	level.Info(logger).Log("Event", "Collecting stats from CB cbemxRebalanceStruct.")
	getCbemxForApi(CBEMXENDPOINT_Rebalance, &cbemxRebalanceStruct.ProgressDetails)
	level.Info(logger).Log("Event", "Collecting stats from CB cbemxClusterUUIDStruct.")
	getCbemxForApi(CBEMXENDPOINT_ClusterUUID, &cbemxClusterUUIDStruct)
	level.Info(logger).Log("Event", "Collecting stats from CB cbemxServerGroupStruct")
	getCbemxForApi(CBEMXENDPOINT_ServerGroups, &cbemxServerGroupStruct)

	exported.cluster_uuid = cbemxClusterUUIDStruct.UUID
	exported.buckets = make(map[int]bucketMetric)
	exported.indexes = make(map[int]indexMetric)
	exported.rebalance_status = make(map[string]float64)
	exported.nodes = make(map[string][]string)
	// bucket stats per bucket
	for i, bucket := range cbemxBucketStatsStructArray.Buckets {
		var tmpBucket bucketMetric
		tmpBucket.bucket_name = bucket.BucketName
		tmpBucket.bucket_replica_count = bucket.VBucketServerMap.NumReplicas
		tmpBucket.bucket_compression_type = bucket.CompressionMode
		tmpBucket.bucket_conflict_resolution = bucket.ConflictResolutionType
		tmpBucket.bucket_eviction_type = bucket.EvictionPolicy
		tmpBucket.bucket_storage_backend = bucket.StorageBackend
		exported.buckets[i] = tmpBucket
	}

	for _, node := range cbemxClusterStatusStruct.Nodes {
		var hostname = strings.Split(node.Hostname, ":")[0]
		exported.nodes[hostname] = node.Services
	}

	// index stats per index
	for i, index := range cbemxIndexStatusStructArray.Indexes {
		// skip replica index definitions due to redundancy
		if index.ReplicaId != 0 {
			continue
		}
		var tmpIndex indexMetric
		tmpIndex.index_name = index.IndexName
		tmpIndex.bucket = index.Bucket
		tmpIndex.scope = index.Scope
		tmpIndex.collection = index.Collection
		if strings.Contains(strings.ToLower(index.Definition), "create primary") {
			tmpIndex.index_type = "primary"
		} else {
			tmpIndex.index_type = "secondary"
		}
		tmpIndex.index_replica_count = index.NumReplicas
		// export index skipping _system indexes
		if tmpIndex.scope != "_system" {
			exported.indexes[i] = tmpIndex
		}

	}

	// count nodes per group
	exported.sever_group_count = len(cbemxServerGroupStruct.Groups)
	for _, group := range cbemxServerGroupStruct.Groups {
		var tmpCount = len(group.Nodes)
		if tmpCount > exported.largest_server_group_count {
			exported.largest_server_group_count = tmpCount
		}
	}

	// cluster status metrics

	exported.cluster_balanced = cbemxClusterStatusStruct.Balanced
	exported.data_memory_quota = cbemxClusterStatusStruct.MemoryQuota
	exported.index_memory_quota = cbemxClusterStatusStruct.IndexMemoryQuota
	exported.ram_quota_used = cbemxClusterStatusStruct.StorageTotals.Ram.QuotaUsed
	exported.index_storage_engine = cbemxIndexSettingsStruct.StorageMode
	exported.slow_queries_threshold = cbemxQuerySettingsStruct.Slow_queries_threshold
	exported.slow_queries_limit = cbemxQuerySettingsStruct.Slow_queries_limit
	//exported.node_count = len(cbemxClusterStatusStruct.Nodes)

	// cluster autofailover settings

	exported.autofailover_current_count = cbemxAutoFailoverStruct.Count
	exported.autofailover_enabled = cbemxAutoFailoverStruct.Enabled
	exported.autofailover_max_count = cbemxAutoFailoverStruct.MaxCount
	exported.autofailover_on_disk_enabled = cbemxAutoFailoverStruct.FailoverOnDataDiskIssues.Enabled
	exported.autofailover_on_disk_timeout = cbemxAutoFailoverStruct.FailoverOnDataDiskIssues.TimePeriod
	exported.autofailover_timeout = cbemxAutoFailoverStruct.Timeout

	// cluster failovers

	exported.failover_complete_counter = cbemxClusterStatusStruct.Counters.Failover_complete
	exported.failover_counter = cbemxClusterStatusStruct.Counters.Failover
	exported.failover_fail_counter = cbemxClusterStatusStruct.Counters.Failover_fail
	exported.failover_start_counter = cbemxClusterStatusStruct.Counters.Failover_start
	exported.failover_stop_counter = cbemxClusterStatusStruct.Counters.Failover_stop
	exported.failover_success_counter = cbemxClusterStatusStruct.Counters.Failover_success

	// cluster rebalances

	exported.rebalance_fail_counter = cbemxClusterStatusStruct.Counters.Rebalance_fail
	exported.rebalance_start_counter = cbemxClusterStatusStruct.Counters.Rebalance_start
	exported.rebalance_stop_counter = cbemxClusterStatusStruct.Counters.Rebalance_stop
	exported.rebalance_success_counter = cbemxClusterStatusStruct.Counters.Rebalance_success

	for host, progress := range cbemxRebalanceStruct.ProgressDetails {

		if host != "status" {
			var progressMap map[string]json.RawMessage
			err := json.Unmarshal(progress, &progressMap)
			if err != nil {
				level.Error(logger).Log("Error unmarshaling %s: %v\n", host, err)
				continue
			}
			for _, value := range progressMap {
				var tmpProgress float64
				err := json.Unmarshal(value, &tmpProgress)
				if err != nil {
					level.Error(logger).Log("Error unmarshaling %s: %v\n", host, err)
					continue
				}
				exported.rebalance_status[host] = tmpProgress
			}

		}

	}

	return
}

// Creating custom metric collector
func metricsCollector() *MetricsCollector {

	level.Info(logger).Log("Event", "Initializing the metrics creation through collector")

	// Defining the ccr metrics
	return &MetricsCollector{
		bucket_replica_count: prometheus.NewDesc("bucket_replica_count",
			"The total number of replicas for a bucket.",
			[]string{"cluster_uuid", "bucket"}, nil,
		),
		bucket_eviction_type: prometheus.NewDesc("bucket_eviction_type",
			"The bucket eviction type {valueOnly/fullEviction/noEviction/nruEviction} selected state(1 - selected).",
			[]string{"cluster_uuid", "bucket", "eviction"}, nil,
		),
		bucket_compression_type: prometheus.NewDesc("bucket_compression_type",
			"The bucket compression type {off/passive/active} selected state(1 - selected).",
			[]string{"cluster_uuid", "bucket", "compression"}, nil,
		),
		bucket_storage_backend: prometheus.NewDesc("bucket_storage_backend",
			"The bucket storage backend type {couchstore/magma/undefined} selected state(1 - selected).",
			[]string{"cluster_uuid", "bucket", "storage_backend"}, nil,
		),
		bucket_conflict_resolution: prometheus.NewDesc("bucket_conflict_resolution",
			"The bucket conflict resolution {seqno/lww/custom} selected state(1 - selected).",
			[]string{"cluster_uuid", "bucket", "conflict_resolution"}, nil,
		),
		index_replica_count: prometheus.NewDesc("index_replica_count",
			"The total number replicas for an index.",
			[]string{"cluster_uuid", "bucket", "scope", "collection", "index_name", "index_type"}, nil,
		),
		index_storage_engine: prometheus.NewDesc("index_storage_engine",
			"Index Storage Engine type {memory optmized / plasma} selected state(1 - selected).",
			[]string{"cluster_uuid", "index_engine"}, nil,
		),
		cluster_balanced: prometheus.NewDesc("cluster_balanced",
			"Cluster balance state 0/1 --> false/true.",
			[]string{"cluster_uuid"}, nil,
		),
		server_group_count: prometheus.NewDesc("server_group_count",
			"Number of server groups in the cluster.",
			[]string{"cluster_uuid"}, nil,
		),
		largest_server_group_count: prometheus.NewDesc("largest_server_group_count",
			"Size of largest server group in the cluster.",
			[]string{"cluster_uuid"}, nil,
		),
		slow_queries_threshold: prometheus.NewDesc("slow_queries_threshold",
			"The threshold for mimnimum query duration in ms for slow query logging.",
			[]string{"cluster_uuid"}, nil,
		),
		slow_queries_limit: prometheus.NewDesc("slow_queries_limit",
			"Retention limit for slow query logging.",
			[]string{"cluster_uuid"}, nil,
		),
		data_memory_quota: prometheus.NewDesc("data_memory_quota",
			"The Data service memory quota in MB.",
			[]string{"cluster_uuid"}, nil,
		),
		index_memory_quota: prometheus.NewDesc("index_memory_quota",
			"The Index service memory quota in MB.",
			[]string{"cluster_uuid"}, nil,
		),
		ram_quota_used: prometheus.NewDesc("ram_quota_used",
			"Total RAM quota used in bytes.",
			[]string{"cluster_uuid"}, nil,
		),
		autofailover_enabled: prometheus.NewDesc("autofailover_enabled",
			"The Autofailover state 0/1 --> disabled/enabled.",
			[]string{"cluster_uuid"}, nil,
		),
		autofailover_timeout: prometheus.NewDesc("autofailover_timeout",
			"The Autofailover timeout in seconds.",
			[]string{"cluster_uuid"}, nil,
		),
		autofailover_on_disk_enabled: prometheus.NewDesc("autofailover_on_disk_enabled",
			"The 'Autofailover On Disk Failures' state 0/1 --> disabled/enabled.",
			[]string{"cluster_uuid"}, nil,
		),
		autofailover_on_disk_timeout: prometheus.NewDesc("autofailover_on_disk_timeout",
			"The 'Autofailover On Disk Failures' timeout in seconds",
			[]string{"cluster_uuid"}, nil,
		),
		autofailover_max_count: prometheus.NewDesc("autofailover_max_count",
			"Maximum count for auto-failed servers.",
			[]string{"cluster_uuid"}, nil,
		),
		autofailover_current_count: prometheus.NewDesc("autofailover_current_count",
			"Current count of auto-failed servers.",
			[]string{"cluster_uuid"}, nil,
		),
		failover_counter: prometheus.NewDesc("failover_counter",
			"The number of failovers performed.",
			[]string{"cluster_uuid"}, nil,
		),
		failover_start_counter: prometheus.NewDesc("failover_start_counter",
			"The total number of failovers started.",
			[]string{"cluster_uuid"}, nil,
		),
		failover_complete_counter: prometheus.NewDesc("failover_complete_counter",
			"The total number of failovers completed.",
			[]string{"cluster_uuidx"}, nil,
		),
		failover_success_counter: prometheus.NewDesc("failover_success_counter",
			"The total number of failovers completed successfully.",
			[]string{"cluster_uuid"}, nil,
		),
		failover_stop_counter: prometheus.NewDesc("failover_stop_counter",
			"The total number of failovers stopped before completion.",
			[]string{"cluster_uuid"}, nil,
		),
		failover_fail_counter: prometheus.NewDesc("failover_fail_counter",
			"The total number of failovers that failed.",
			[]string{"cluster_uuid"}, nil,
		),
		rebalance_start_counter: prometheus.NewDesc("rebalance_start_counter",
			"The total number of rebalances started.",
			[]string{"cluster_uuid"}, nil,
		),
		rebalance_success_counter: prometheus.NewDesc("rebalance_success_counter",
			"The total number of rebalances completed successfully.",
			[]string{"cluster_uuid"}, nil,
		),
		rebalance_fail_counter: prometheus.NewDesc("rebalance_fail_counter",
			"The total number of rebalances that failed.",
			[]string{"cluster_uuid"}, nil,
		),
		rebalance_stop_counter: prometheus.NewDesc("rebalance_stop_counter",
			"The total number of rebalances stopped before completion.",
			[]string{"cluster_uuid"}, nil,
		),
		rebalance_status: prometheus.NewDesc("rebalance_status",
			"The current rebalance progress per node.",
			[]string{"cluster_uuid", "node", "services"}, nil,
		),
	}
}

// Defining the channel collection for the custom collector
func (collector *MetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.autofailover_current_count
	ch <- collector.autofailover_enabled
	ch <- collector.autofailover_max_count
	ch <- collector.autofailover_on_disk_enabled
	ch <- collector.autofailover_on_disk_timeout
	ch <- collector.autofailover_timeout
	ch <- collector.bucket_compression_type
	ch <- collector.bucket_conflict_resolution
	ch <- collector.bucket_eviction_type
	ch <- collector.bucket_replica_count
	ch <- collector.bucket_storage_backend
	ch <- collector.cluster_balanced
	ch <- collector.data_memory_quota
	ch <- collector.failover_complete_counter
	ch <- collector.failover_counter
	ch <- collector.failover_fail_counter
	ch <- collector.rebalance_start_counter
	ch <- collector.failover_stop_counter
	ch <- collector.failover_success_counter
	ch <- collector.index_memory_quota
	ch <- collector.index_replica_count
	ch <- collector.index_storage_engine
	ch <- collector.ram_quota_used
	ch <- collector.rebalance_fail_counter
	ch <- collector.rebalance_start_counter
	ch <- collector.rebalance_status
	ch <- collector.rebalance_stop_counter
	ch <- collector.rebalance_success_counter
	ch <- collector.slow_queries_limit
	ch <- collector.slow_queries_threshold
	ch <- collector.server_group_count
	ch <- collector.largest_server_group_count

}

// Label options for radio select metric streams
var INDEX_STORAGE_ENGINES = [...]string{"memory_optimize", "plasma"}
var BUCKET_EVICTION_METHOD = [...]string{"valueOnly", "fullEviction", "noEviction", "nruEviction"}
var BUCKET_COMPRESSION_METHOD = [...]string{"off", "passive", "active"}
var BUCKET_STORAGE_BACKEND = [...]string{"couchstore", "magma", "undefined"}
var BUCKET_CONFLICT_RESOLUTION = [...]string{"seqno", "lww", "custom"}

func boolVal(toConvert bool) int8 {
	if toConvert {
		return 1
	} else {
		return 0
	}
}

/*
Implementing the channel collection of metrics for the custom collector
Generates the Prometheus formatted metrics output.
*/
func (collector *MetricsCollector) Collect(ch chan<- prometheus.Metric) {
	//Throttle calls to once every THROTTLE_TIME seconds
	var throttle_time, err = strconv.Atoi(os.Getenv("EMX_THROTTLE_TIME"))
	if err != nil {
		throttle_time = EMX_THROTTLE_TIME
	}
	if time.Since(lastCallTime) < time.Duration(throttle_time)*time.Second {
		level.Error(logger).Log("Error", "Less than "+strconv.Itoa(throttle_time)+" seconds between scrape attempts. Last call before "+time.Since(lastCallTime).String()+" seconds.")
		return
	}
	lastCallTime = time.Now()

	// Getting the Couchbase cluster and and its related details
	setCBConnectionString()
	level.Info(logger).Log("Couchbase API URL", CB_CONNECTIONSTRING)
	level.Info(logger).Log("Event", "Fetching the EMX stats details of couchbase host")
	res := getCbemxStats()

	var uuid = res.cluster_uuid

	level.Info(logger).Log("Event", "Generating metrics for Couchbase EMX.")

	// cluster lever metrics
	for _, option := range INDEX_STORAGE_ENGINES {
		ch <- prometheus.MustNewConstMetric(collector.index_storage_engine, prometheus.GaugeValue, float64(boolVal(option == res.index_storage_engine)), uuid, option)
	}
	ch <- prometheus.MustNewConstMetric(collector.cluster_balanced, prometheus.GaugeValue, float64(boolVal(res.cluster_balanced)), uuid)
	ch <- prometheus.MustNewConstMetric(collector.slow_queries_threshold, prometheus.GaugeValue, float64(res.slow_queries_threshold), uuid)
	ch <- prometheus.MustNewConstMetric(collector.slow_queries_limit, prometheus.GaugeValue, float64(res.slow_queries_limit), uuid)
	ch <- prometheus.MustNewConstMetric(collector.data_memory_quota, prometheus.GaugeValue, float64(res.data_memory_quota), uuid)
	ch <- prometheus.MustNewConstMetric(collector.index_memory_quota, prometheus.GaugeValue, float64(res.index_memory_quota), uuid)
	ch <- prometheus.MustNewConstMetric(collector.ram_quota_used, prometheus.GaugeValue, float64(res.ram_quota_used), uuid)
	ch <- prometheus.MustNewConstMetric(collector.server_group_count, prometheus.GaugeValue, float64(res.sever_group_count), uuid)
	ch <- prometheus.MustNewConstMetric(collector.largest_server_group_count, prometheus.GaugeValue, float64(res.largest_server_group_count), uuid)

	// Autofailover
	ch <- prometheus.MustNewConstMetric(collector.autofailover_enabled, prometheus.GaugeValue, float64(boolVal(res.autofailover_enabled)), uuid)
	ch <- prometheus.MustNewConstMetric(collector.autofailover_timeout, prometheus.GaugeValue, float64(res.autofailover_timeout), uuid)
	ch <- prometheus.MustNewConstMetric(collector.autofailover_on_disk_enabled, prometheus.GaugeValue, float64(boolVal(res.autofailover_on_disk_enabled)), uuid)
	ch <- prometheus.MustNewConstMetric(collector.autofailover_on_disk_timeout, prometheus.GaugeValue, float64(res.autofailover_on_disk_timeout), uuid)
	ch <- prometheus.MustNewConstMetric(collector.autofailover_max_count, prometheus.GaugeValue, float64(res.autofailover_max_count), uuid)
	ch <- prometheus.MustNewConstMetric(collector.autofailover_current_count, prometheus.CounterValue, float64(res.autofailover_current_count), uuid)
	// Failovers
	ch <- prometheus.MustNewConstMetric(collector.failover_counter, prometheus.CounterValue, float64(res.failover_counter), uuid)
	ch <- prometheus.MustNewConstMetric(collector.failover_start_counter, prometheus.CounterValue, float64(res.failover_start_counter), uuid)
	ch <- prometheus.MustNewConstMetric(collector.failover_complete_counter, prometheus.CounterValue, float64(res.failover_complete_counter), uuid)
	ch <- prometheus.MustNewConstMetric(collector.failover_success_counter, prometheus.CounterValue, float64(res.failover_success_counter), uuid)
	ch <- prometheus.MustNewConstMetric(collector.failover_stop_counter, prometheus.CounterValue, float64(res.failover_stop_counter), uuid)
	ch <- prometheus.MustNewConstMetric(collector.failover_fail_counter, prometheus.CounterValue, float64(res.failover_fail_counter), uuid)
	// Rebalance
	ch <- prometheus.MustNewConstMetric(collector.rebalance_start_counter, prometheus.CounterValue, float64(res.rebalance_start_counter), uuid)
	ch <- prometheus.MustNewConstMetric(collector.rebalance_success_counter, prometheus.CounterValue, float64(res.rebalance_success_counter), uuid)
	ch <- prometheus.MustNewConstMetric(collector.rebalance_fail_counter, prometheus.CounterValue, float64(res.rebalance_fail_counter), uuid)
	ch <- prometheus.MustNewConstMetric(collector.rebalance_stop_counter, prometheus.CounterValue, float64(res.rebalance_stop_counter), uuid)
	for host, progress := range res.rebalance_status {
		var hostname = strings.Split(host, "@")[1]
		var services string = strings.Join(res.nodes[hostname], ",")
		ch <- prometheus.MustNewConstMetric(collector.rebalance_status, prometheus.GaugeValue, float64(progress), uuid, host, services)
	}
	// per bucket metrics
	for _, bucket := range res.buckets {
		ch <- prometheus.MustNewConstMetric(collector.bucket_replica_count, prometheus.GaugeValue, float64(bucket.bucket_replica_count), uuid, bucket.bucket_name)
		for _, ev := range BUCKET_EVICTION_METHOD {
			ch <- prometheus.MustNewConstMetric(collector.bucket_eviction_type, prometheus.GaugeValue, float64(boolVal(ev == bucket.bucket_eviction_type)), uuid, bucket.bucket_name, ev)
		}
		for _, com := range BUCKET_COMPRESSION_METHOD {
			ch <- prometheus.MustNewConstMetric(collector.bucket_compression_type, prometheus.GaugeValue, float64(boolVal(com == bucket.bucket_compression_type)), uuid, bucket.bucket_name, com)
		}
		for _, sto := range BUCKET_STORAGE_BACKEND {
			ch <- prometheus.MustNewConstMetric(collector.bucket_storage_backend, prometheus.GaugeValue, float64(boolVal(sto == bucket.bucket_storage_backend)), uuid, bucket.bucket_name, sto)
		}
		for _, con := range BUCKET_CONFLICT_RESOLUTION {
			ch <- prometheus.MustNewConstMetric(collector.bucket_conflict_resolution, prometheus.GaugeValue, float64(boolVal(con == bucket.bucket_conflict_resolution)), uuid, bucket.bucket_name, con)
		}
	}

	// per index metrics
	for _, idx := range res.indexes {
		ch <- prometheus.MustNewConstMetric(collector.index_replica_count, prometheus.GaugeValue, float64(idx.index_replica_count), uuid, idx.bucket, idx.scope, idx.collection, idx.index_name, idx.index_type)

	}

	level.Info(logger).Log("Event", "Channelled all the metrics to the collector")

}

func CreateCouchbaseEMXStatsMetrics(logger log.Logger, tlsConfig utility.TLSConfig) {
	var tlsKey = os.Getenv("CB_CLIENT_KEY")
	if tlsKey == "" {
		tlsKey = tlsConfig.TlsKeyPath
	}
	var tlsCert = os.Getenv("CB_CLIENT_CERT")
	if tlsCert == "" {
		tlsCert = tlsConfig.TlsCertificatePath
	}
	cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
	if err != nil {
		level.Error(logger).Log("Error loading client certificate", err)
		return
	}
	X509KeyPair = cert
	prometheus.MustRegister(metricsCollector())
	level.Info(logger).Log("Event", "Successfully registered the metrics with prometheus")

}
