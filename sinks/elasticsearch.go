package sinks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	elasticsearch "github.com/elastic/go-elasticsearch/v7"
	esapi "github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/webdevops/pagerduty2es/exporter"
)

type ElasticsearchPusher struct {
	client *elasticsearch.Client
}

func NewElasticsearchPusher(client *elasticsearch.Client) *ElasticsearchPusher {
	return &ElasticsearchPusher{
		client: client,
	}
}

func (ep *ElasticsearchPusher) Push(data []byte) error {
	// Implement Elasticsearch data push logic here
	// Use the 'data' to push data to Elasticsearch
	// Example: _, err := ep.client.Index(...)

	return nil
}

type (
	PagerdutyElasticsearchExporter struct {
		scrapeTime *time.Duration

		elasticSearchClient     *elasticsearch.Client
		elasticsearchIndexName  string
		elasticsearchBatchCount int
		elasticsearchRetryCount int
		elasticsearchRetryDelay time.Duration

		pagerdutyClient    *pagerduty.Client
		pagerdutyDateRange *time.Duration

		prometheus struct {
			incident         *prometheus.CounterVec
			incidentLogEntry *prometheus.CounterVec
			esRequestTotal   *prometheus.CounterVec
			esRequestRetries *prometheus.CounterVec
			duration         *prometheus.GaugeVec
		}
	}
)

func (e *PagerdutyElasticsearchExporter) Init() {
	e.elasticsearchBatchCount = 10
	e.elasticsearchRetryCount = 5
	e.elasticsearchRetryDelay = 5 * time.Second

	e.prometheus.incident = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pagerduty2es_incident_total",
			Help: "PagerDuty2es incident counter",
		},
		[]string{},
	)

	e.prometheus.incidentLogEntry = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pagerduty2es_incident_logentry_total",
			Help: "PagerDuty2es incident logentry counter",
		},
		[]string{},
	)

	e.prometheus.esRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pagerduty2es_elasticsearch_requet_total",
			Help: "PagerDuty2es elasticsearch request total counter",
		},
		[]string{},
	)
	e.prometheus.esRequestRetries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pagerduty2es_elasticsearch_request_retries",
			Help: "PagerDuty2es elasticsearch request retries counter",
		},
		[]string{},
	)

	e.prometheus.duration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty2es_duration",
			Help: "PagerDuty2es duration",
		},
		[]string{},
	)

	prometheus.MustRegister(e.prometheus.incident)
	prometheus.MustRegister(e.prometheus.incidentLogEntry)
	prometheus.MustRegister(e.prometheus.esRequestTotal)
	prometheus.MustRegister(e.prometheus.esRequestRetries)
	prometheus.MustRegister(e.prometheus.duration)
}

func (e *PagerdutyElasticsearchExporter) SetScrapeTime(value time.Duration) {
	e.scrapeTime = &value
}

func (e *PagerdutyElasticsearchExporter) ConnectPagerduty(token string, httpClient *http.Client) {
	e.pagerdutyClient = pagerduty.NewClient(token)
	e.pagerdutyClient.HTTPClient = httpClient
}

func (e *PagerdutyElasticsearchExporter) SetPagerdutyDateRange(value time.Duration) {
	e.pagerdutyDateRange = &value
}

func (e *PagerdutyElasticsearchExporter) ConnectElasticsearch(cfg elasticsearch.Config, indexName string) {
	var err error
	e.elasticSearchClient, err = elasticsearch.NewClient(cfg)
	if err != nil {
		panic(err)
	}

	tries := 0
	for {
		_, err = e.elasticSearchClient.Info()
		if err != nil {
			tries++
			if tries >= 5 {
				panic(err)
			} else {
				log.Info("failed to connect to ES, retry...")
				time.Sleep(5 * time.Second)
				continue
			}
		}

		break
	}

	e.elasticsearchIndexName = indexName
}

func (e *PagerdutyElasticsearchExporter) SetElasticsearchBatchCount(batchCount int) {
	e.elasticsearchBatchCount = batchCount
}

func (e *PagerdutyElasticsearchExporter) SetElasticsearchRetry(retryCount int, retryDelay time.Duration) {
	e.elasticsearchRetryCount = retryCount
	e.elasticsearchRetryDelay = retryDelay
}

func (e *PagerdutyElasticsearchExporter) indexIncident(incident pagerduty.Incident, callback chan<- *esapi.IndexRequest) {
	e.prometheus.incident.WithLabelValues().Inc()

	createTime, err := time.Parse(time.RFC3339, incident.CreatedAt)
	if err != nil {
		panic(err)
	}

	pusherIncident := exporter.PusherIncident{
		Timestamp:  createTime.Format(time.RFC3339),
		IncidentId: incident.ID,
		Incident:   &incident,
	}
	incidentJson, _ := json.Marshal(pusherIncident)

	req := esapi.IndexRequest{
		Index:      e.buildIndexName(createTime),
		DocumentID: fmt.Sprintf("incident-%v", incident.ID),
		Body:       bytes.NewReader(incidentJson),
	}
	callback <- &req
}

func (e *PagerdutyElasticsearchExporter) buildIndexName(createTime time.Time) string {
	ret := e.elasticsearchIndexName

	ret = strings.Replace(ret, "%y", createTime.Format("2006"), -1)
	ret = strings.Replace(ret, "%m", createTime.Format("01"), -1)
	ret = strings.Replace(ret, "%d", createTime.Format("02"), -1)

	return ret
}

func (e *PagerdutyElasticsearchExporter) indexIncidentLogEntry(incident pagerduty.Incident, logEntry pagerduty.LogEntry, callback chan<- *esapi.IndexRequest) {
	e.prometheus.incidentLogEntry.WithLabelValues().Inc()

	createTime, err := time.Parse(time.RFC3339, logEntry.CreatedAt)
	if err != nil {
		panic(err)
	}

	pusherLogEntry := exporter.PusherIncidentLog{
		Timestamp:  createTime.Format(time.RFC3339),
		IncidentId: incident.ID,
		LogEntry:   &logEntry,
	}
	logEntryJson, _ := json.Marshal(pusherLogEntry)

	req := esapi.IndexRequest{
		Index:      e.buildIndexName(createTime),
		DocumentID: fmt.Sprintf("logentry-%v", logEntry.ID),
		Body:       bytes.NewReader(logEntryJson),
	}
	callback <- &req
}

type (
	BulkMetaIndex struct {
		Index BulkMetaIndexIndex `json:"index,omitempty"`
	}

	BulkMetaIndexIndex struct {
		Id    string `json:"_id,omitempty"`
		Type  string `json:"_type,omitempty"`
		Index string `json:"_index,omitempty"`
	}
)

func (e *PagerdutyElasticsearchExporter) doESIndexRequestBulk(bulkRequests []*esapi.IndexRequest) {
	var buf bytes.Buffer
	newline := []byte("\n")

	var err error
	var resp *esapi.Response

	for i := 0; i < e.elasticsearchRetryCount; i++ {
		for _, indexRequest := range bulkRequests {
			// generate bulk index action line
			meta := BulkMetaIndex{
				Index: BulkMetaIndexIndex{
					Id:    indexRequest.DocumentID,
					Type:  indexRequest.DocumentType,
					Index: indexRequest.Index,
				},
			}
			metaJson, _ := json.Marshal(meta)

			// generate document line
			document := new(bytes.Buffer)
			_, readErr := document.ReadFrom(indexRequest.Body)
			if readErr != nil {
				panic(readErr)
			}

			// generate index line
			buf.Grow(len(metaJson) + len(newline) + document.Len() + len(newline))
			buf.Write(metaJson)
			buf.Write(newline)
			buf.Write(document.Bytes())
			buf.Write(newline)
		}

		e.prometheus.esRequestTotal.WithLabelValues().Inc()
		resp, err = e.elasticSearchClient.Bulk(bytes.NewReader(buf.Bytes()))
		if err == nil && resp.StatusCode == http.StatusOK {
			if err := resp.Body.Close(); err != nil {
				log.Errorf(err.Error())
			}

			// success
			return
		}

		if resp != nil {
			log.Errorf("unexpected HTTP %v response: %v", resp.StatusCode, resp.String())
		}

		// got an error
		log.Errorf("retrying ES index error: %v", err)
		e.prometheus.esRequestRetries.WithLabelValues().Inc()

		// wait until retry
		time.Sleep(e.elasticsearchRetryDelay)
	}

	// must be an error
	if err != nil {
		log.Panicf("fatal ES index error: %v", err)
	} else {
		panic("Unable to process ES request")
	}
}
