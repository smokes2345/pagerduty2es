package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/util/workqueue"
)

type (
	PagerdutyEventSource struct {
		pagerdutyClient    *pagerduty.Client
		pagerdutyDateRange *time.Duration
		ctx                context.Context
		lastEvent          *PagerdutyEvent
		Checkpoint         *PagerdutyEventSourceCheckpoint
		Name               string
	}
	PagerdutyEvent struct {
		Incident *pagerduty.Incident
		Source   PagerdutyEventSource
		// PushedCB func(string)
	}
	PagerdutyEventSourceCheckpoint struct {
		Database *dynamodb.DynamoDB
		Table    string
		Key      string
	}
)

func (pde PagerdutyEvent) Id() string {
	return fmt.Sprintf("%s-%s", pde.Source.Name, pde.Incident.ID)
}

func (pde *PagerdutyEvent) String() string {
	return pde.Id()
}

// func (pde *PagerdutyEvent) Data() *pagerduty.Incident {
// 	return pde.Event
// }

func (pde PagerdutyEvent) Data() []byte {
	pde_json, err := json.Marshal(pde.Incident)
	if err != nil {
		log.Warnf("Error decoding event %s: %s", pde.Id(), err)
	}
	return []byte(pde_json)
}

func (pde PagerdutyEvent) PushedCB() func(string) {
	return pde.Source.EventProcessedCallback
}

func (e *PagerdutyEventSource) EventProcessedCallback(event_id string) {
	key := make(map[string]*dynamodb.AttributeValue)
	key_val := dynamodb.AttributeValue{S: &e.Name}
	key["source"] = &key_val
	update := dynamodb.UpdateItemInput{
		TableName: &e.Checkpoint.Table,
		Key:       key,
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":i": {
				S: aws.String(event_id),
			},
			":s": {
				S: aws.String(e.Name),
			},
		},
		UpdateExpression: aws.String("set checkpoint = :i where source = :s"),
	}
	_, err := e.Checkpoint.Database.UpdateItem(&update)
	if err != nil {
		log.Warningf("Could not checkpoint PD event %s", event_id)
	}
}

func (e *PagerdutyEventSource) Init(token string, window time.Duration, httpClient *http.Client) {
	e.ctx = context.Background()
	e.ConnectPagerduty(token, httpClient)
	e.pagerdutyDateRange = &window
	aws_client := session.Must(session.NewSession())
	e.Checkpoint = &PagerdutyEventSourceCheckpoint{
		Database: dynamodb.New(aws_client),
		Table:    "pagerduty_checkpoint",
	}
	tableCreateInput := dynamodb.CreateTableInput{
		TableName: &e.Checkpoint.Table,
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("source"),
				AttributeType: aws.String("S"),
			},
			// {
			// 	AttributeName: aws.String("checkpoint"),
			// 	AttributeType: aws.String("S"),
			// },
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("source"),
				KeyType:       aws.String("HASH"),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(10),
			WriteCapacityUnits: aws.Int64(10),
		},
	}

	_, err := e.Checkpoint.Database.DescribeTable(&dynamodb.DescribeTableInput{TableName: &e.Checkpoint.Table})
	if err != nil {
		log.Warningf("Error retriving table %s: %s", e.Checkpoint.Table, err)
		_, err = e.Checkpoint.Database.CreateTable(&tableCreateInput)
		if err != nil {
			log.Fatalf("Could not create table %s", e.Checkpoint.Table)
		}
	}
	// user, _ := e.pagerdutyClient.GetCurrentUserWithContext(e.ctx, pagerduty.GetCurrentUserOptions{})
	// e.Name = user.Name
}

func (e *PagerdutyEventSource) ConnectPagerduty(token string, httpClient *http.Client) {
	e.pagerdutyClient = pagerduty.NewClient(token)
	e.pagerdutyClient.HTTPClient = httpClient
}

func (e *PagerdutyEventSource) ScrapeEvents(queue *workqueue.Type) {

	since := time.Now().Add(-*e.pagerdutyDateRange).Format(time.RFC3339)
	if e.lastEvent != nil {
		since = e.lastEvent.Incident.CreatedAt
	}
	listOpts := pagerduty.ListIncidentsOptions{
		Since: since,
	}
	listOpts.Limit = PagerdutyIncidentLimit
	listOpts.Offset = 0
	ctx := context.Background()

	for {

		incidentResponse, err := e.pagerdutyClient.ListIncidentsWithContext(ctx, listOpts)
		if err != nil {
			panic(err)
		}

		for _, incident := range incidentResponse.Incidents {
			if e.lastEvent != nil {
				if e.lastEvent.Incident.IncidentKey == incident.IncidentKey {
					continue
				}
			}
			// workaround for https://github.com/PagerDuty/go-pagerduty/issues/218
			contextLogger := log.WithField("incident", incident.ID)
			pd_event := PagerdutyEvent{&incident, *e}
			// e.indexIncident(incident, esIndexRequestChannel)

			// listLogOpts := pagerduty.ListIncidentLogEntriesOptions{}
			// incidentLogResponse, err := e.pagerdutyClient.ListIncidentLogEntriesWithContext(ctx, incident.ID, listLogOpts)
			// if err != nil {
			// 	panic(err)
			// }

			// for _, logEntry := range incidentLogResponse.LogEntries {
			// 	contextLogger.WithField("logEntry", logEntry.ID).Debugf("logEntry %v", logEntry.ID)
			// 	// e.indexIncidentLogEntry(incident, logEntry, esIndexRequestChannel)
			// }

			queue.Add(pd_event)
			contextLogger.Debug("queued")
			e.lastEvent = &pd_event
		}

		if !incidentResponse.More {
			break
		}
		listOpts.Offset += listOpts.Limit
	}
}
