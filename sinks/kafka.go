package sinks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"
	"github.com/sirupsen/logrus"
	"github.com/webdevops/pagerduty2es/exporter"
)

type KafkaSink struct {
	client kafka.Writer
	opts   *KafkaSinkOpts
}

type KafkaSinkOpts struct {
	Addr      string
	Timeout   time.Duration
	Transport kafka.Transport
	Topic     string
	Context   *context.Context
	Username  string
	Password  string
}

func (k *KafkaSink) Init(opts *KafkaSinkOpts) {
	sasl_mech, _ := scram.Mechanism(scram.SHA512, opts.Username, opts.Password)
	opts.Transport = kafka.Transport{
		TLS:  k.TlsConfig(opts),
		SASL: sasl_mech,
	}
	// k.client = kafka.Writer{
	// 	Addr:      opts.Addr,
	// 	Transport: &opts.Transport,
	// }
	k.client = kafka.Writer{
		Addr:                   kafka.TCP(opts.Addr),
		Topic:                  opts.Topic,
		Balancer:               &kafka.LeastBytes{},
		Transport:              &opts.Transport,
		AllowAutoTopicCreation: true,
	}
	k.opts = opts
}

func (k *KafkaSink) TlsConfig(opts *KafkaSinkOpts) *tls.Config {
	tlsc := tls.Config{
		InsecureSkipVerify: true,
	}
	return &tlsc
}

func (k *KafkaSink) Push(data exporter.Event) error {
	incidentBytes := data.Data()
	var event exporter.Event
	json.Unmarshal(incidentBytes, event)
	var incident pagerduty.Incident
	json.Unmarshal(event.Data(), &incident)
	inc_json, _ := json.Marshal(incidentBytes)
	logrus.Debugf("Pushing event %s to %s/%s", incident.IncidentKey, k.client.Addr, k.client.Topic)
	err := k.client.WriteMessages(
		*k.opts.Context,
		kafka.Message{
			Key:   []byte("message"),
			Value: inc_json,
		},
	)
	return err
}
