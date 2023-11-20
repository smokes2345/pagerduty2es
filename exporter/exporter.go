package exporter

import (
	"sync"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/pagerduty2es/sources"
	"k8s.io/client-go/util/workqueue"
)

const MACHINE_RUN = 1
const MACHINE_STOP = 0

type Event interface {
	//Id() string
	//String() string
	//Json() []byte
	Data() []byte
}

type EventSource interface {
	ScrapeEvents(*workqueue.Type)
	EventProcessedCallback(string)
}

type DataPusher interface {
	Push(data Event) error
}

type PusherIncident struct {
	DocumentID string `json:"_id,omitempty"`
	Timestamp  string `json:"@timestamp,omitempty"`
	IncidentId string `json:"@incident,omitempty"`
	*pagerduty.Incident
}

type PusherIncidentLog struct {
	DocumentID string `json:"_id,omitempty"`
	Timestamp  string `json:"@timestamp,omitempty"`
	IncidentId string `json:"@incident,omitempty"`
	*pagerduty.LogEntry
}

type Exporter struct {
	Sources            []EventSource
	Sinks              []DataPusher
	ScrapeTime         time.Duration
	PagerdutyDateRange int
	Queue              workqueue.Type
	BookmarkDB         dynamodb.DynamoDB
}

func (e *Exporter) queueWriter() bool {
	return func() bool {
		event, quit := e.Queue.Get()
		if quit {
			return false
		}

		for _, sink := range e.Sinks {
			log.Debugf("writing to %s to %T", event, sink)

		}
		return true
	}()
}

func (e *Exporter) RunSingle() {
	go e.processItems()
	e.runScrape()
}

func (e *Exporter) RunDaemon() {
	go e.processItems()
	go func() {
		for {
			e.runScrape()
			e.sleepUntilNextCollection()
		}
	}()
}

func (e *Exporter) sleepUntilNextCollection() {
	log.Debugf("sleeping %v", e.ScrapeTime)
	time.Sleep(time.Duration(e.ScrapeTime))
}

func (e *Exporter) runScrape() {
	var wgProcess sync.WaitGroup
	log.Info("starting scrape")

	// since := time.Now().Add(-*e.pagerdutyDateRange).Format(time.RFC3339)
	// listOpts := pagerduty.ListIncidentsOptions{
	// 	Since: since,
	// }
	// listOpts.Limit = PagerdutyIncidentLimit
	// listOpts.Offset = 0

	// esIndexRequestChannel := make(chan *esapi.IndexRequest, e.elasticsearchBatchCount)

	startTime := time.Now()

	// // index from channel
	wgProcess.Add(1)
	go func() {
		defer wgProcess.Done()

		for _, src := range e.Sources {
			go src.ScrapeEvents(&e.Queue)
		}
		// 	bulkIndexRequests := []*esapi.IndexRequest{}
		// 	for esIndexRequest := range esIndexRequestChannel {
		// 		bulkIndexRequests = append(bulkIndexRequests, esIndexRequest)

		// 		if len(bulkIndexRequests) >= e.elasticsearchBatchCount {
		// 			e.doESIndexRequestBulk(bulkIndexRequests)
		// 			bulkIndexRequests = []*esapi.IndexRequest{}
		// 		}
		// 	}

		// 	if len(bulkIndexRequests) >= 1 {
		// 		e.doESIndexRequestBulk(bulkIndexRequests)
		// 	}
	}()

	// for {
	// ctx := context.Background()
	// incidentResponse, err := e.pagerdutyClient.ListIncidentsWithContext(ctx, listOpts)
	// if err != nil {
	// 	panic(err)
	// }

	// for _, incident := range incidentResponse.Incidents {
	// 		// workaround for https://github.com/PagerDuty/go-pagerduty/issues/218
	// 		contextLogger := log.WithField("incident", incident.ID)

	// 		contextLogger.Debugf("incident %v", incident.ID)
	// 		e.indexIncident(incident, esIndexRequestChannel)

	// 		listLogOpts := pagerduty.ListIncidentLogEntriesOptions{}
	// 		incidentLogResponse, err := e.pagerdutyClient.ListIncidentLogEntriesWithContext(ctx, incident.ID, listLogOpts)
	// 		if err != nil {
	// 			panic(err)
	// 		}

	// 		for _, logEntry := range incidentLogResponse.LogEntries {
	// 			contextLogger.WithField("logEntry", logEntry.ID).Debugf("logEntry %v", logEntry.ID)
	// 			e.indexIncidentLogEntry(incident, logEntry, esIndexRequestChannel)
	// 		}
	// 	}

	// 	if !incidentResponse.More {
	// 		break
	// 	}
	// 	listOpts.Offset += listOpts.Limit
	// }
	// close(esIndexRequestChannel)

	wgProcess.Wait()

	duration := time.Since(startTime)
	// e.prometheus.duration.WithLabelValues().Set(duration.Seconds())
	log.WithField("duration", duration.String()).Info("finished scraping")
}

func (e *Exporter) processItems() bool {
	for {
		event, shutdown := e.Queue.Get()

		//bytes, _ := json.Marshal(event)

		for _, s := range e.Sinks {
			err := s.Push(event.(sources.PagerdutyEvent))
			if err != nil {
				log.Warnf("Error pushing to %T: %s", s, err)
			}
		}

		e.Queue.Done(event)

		for _, src := range e.Sources {
			src.EventProcessedCallback(event.(*sources.PagerdutyEvent).Id())
		}

		if shutdown {
			return true
		}
	}
}
