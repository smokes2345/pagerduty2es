package sources

import "k8s.io/client-go/util/workqueue"

const PagerdutyIncidentLimit = 100

type Event interface {
	Id(string)
	String(string)
	Json([]byte)
	EventProcessedCallback(string)
}

type EventSource interface {
	ScrapeEvents(*workqueue.Type)
}
