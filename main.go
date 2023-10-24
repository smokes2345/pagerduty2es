package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/util/workqueue"

	"github.com/webdevops/pagerduty2es/config"
	"github.com/webdevops/pagerduty2es/exporter"
	"github.com/webdevops/pagerduty2es/sinks"
	"github.com/webdevops/pagerduty2es/sources"
)

const (
	author = "webdevops.io"

	// Limit of pagerduty incidents per call
	PagerdutyIncidentLimit = 100
)

var (
	e         exporter.Exporter
	argparser *flags.Parser
	opts      config.Opts
	outs      []exporter.DataPusher

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"
)

func main() {
	initArgparser()

	log.Infof("starting pagerduty2es v%s (%s; %s; by %v)", gitTag, gitCommit, runtime.Version(), author)
	log.Info(string(opts.GetJson()))

	e = exporter.Exporter{}
	e.ScrapeTime = opts.ScrapeTime
	e.Queue = *workqueue.New()

	if len(opts.Kafka.Address) > 0 {
		ctx := context.Background()
		ks := sinks.KafkaSink{}
		ks.Init(&sinks.KafkaSinkOpts{
			Addr:     opts.Kafka.Address,
			Context:  &ctx,
			Topic:    "lma_pagerduty",
			Username: opts.Kafka.Username,
			Password: opts.Kafka.Password,
		})
		e.Sinks = append(e.Sinks, &ks)
	}

	// log.Infof("init exporter")
	// exporter := PagerdutyElasticsearchExporter{}
	// exporter.Init()
	// exporter.SetScrapeTime(opts.ScrapeTime)
	// exporter.SetPagerdutyDateRange(opts.PagerDuty.Since)
	// exporter.ConnectPagerduty(
	// 	opts.PagerDuty.AuthToken,
	// 	&http.Client{
	// 		Transport: &http.Transport{
	// 			Proxy: http.ProxyFromEnvironment,
	// 			DialContext: (&net.Dialer{
	// 				Timeout:   30 * time.Second,
	// 				KeepAlive: 30 * time.Second,
	// 			}).DialContext,
	// 			MaxConnsPerHost:       opts.PagerDuty.MaxConnections,
	// 			MaxIdleConns:          opts.PagerDuty.MaxConnections,
	// 			IdleConnTimeout:       60 * time.Second,
	// 			TLSHandshakeTimeout:   10 * time.Second,
	// 			ExpectContinueTimeout: 1 * time.Second,
	// 			MaxIdleConnsPerHost:   runtime.GOMAXPROCS(0) + 1,
	// 		},
	// 	},
	// )

	// cfg := elasticsearch.Config{
	// 	Addresses: opts.Elasticsearch.Addresses,
	// 	Username:  opts.Elasticsearch.Username,
	// 	Password:  opts.Elasticsearch.Password,
	// 	APIKey:    opts.Elasticsearch.ApiKey,
	// 	Transport: &http.Transport{
	// 		Proxy: http.ProxyFromEnvironment,
	// 	},
	// }
	// exporter.ConnectElasticsearch(cfg, opts.Elasticsearch.Index)
	// exporter.SetElasticsearchBatchCount(opts.Elasticsearch.BatchCount)
	// exporter.SetElasticsearchRetry(opts.Elasticsearch.RetryCount, opts.Elasticsearch.RetryDelay)

	// if opts.ScrapeTime.Seconds() > 0 {
	// 	log.Infof("starting daemon run")
	// 	exporter.RunDaemon()

	// 	// daemon mode
	// 	log.Infof("starting http server on %s", opts.ServerBind)
	// 	startHttpServer()
	// } else {
	// 	log.Infof("starting single run")
	// 	exporter.RunSingle()
	// 	log.Infof("completed single run")
	// }
	pd := sources.PagerdutyEventSource{}
	pd.Init(opts.PagerDuty.AuthToken, opts.PagerDuty.Since, http.DefaultClient)
	e.Sources = append(e.Sources, &pd)
	//ctx := context.Background()
	//workqueue.ParallelizeUntil(ctx, 4, pieces, doWorkPiece, opts)
	e.RunDaemon()
	startHttpServer()
}

// init argparser and parse/validate arguments
func initArgparser() {
	argparser = flags.NewParser(&opts, flags.Default)
	_, err := argparser.Parse()

	// check if there is an parse error
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}

	// verbose level
	if opts.Logger.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	// debug level
	if opts.Logger.Debug {
		log.SetReportCaller(true)
		log.SetLevel(log.TraceLevel)
		log.SetFormatter(&log.TextFormatter{
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, ".")
				funcName := s[len(s)-1]
				return funcName, fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
			},
		})
	}

	// json log format
	if opts.Logger.LogJson {
		log.SetReportCaller(true)
		log.SetFormatter(&log.JSONFormatter{
			DisableTimestamp: true,
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, ".")
				funcName := s[len(s)-1]
				return funcName, fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
			},
		})
	}
}

// start and handle prometheus handler
func startHttpServer() {
	// healthz
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			log.Error(err)
		}
	})

	http.Handle("/metrics", promhttp.Handler())
	log.Infof("Starting server on %s", opts.ServerBind)
	log.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}
