Conversion to write to kafka underway

TODO:
- Implement checkpoint mechanism to keep track of last processed event
- update docker image
- implement helm chart


PagerDuty2es (elasticsearch) exporter
=====================================

[![license](https://img.shields.io/github/license/webdevops/pagerduty2es.svg)](https://github.com/webdevops/pagerduty2es/blob/master/LICENSE)
[![DockerHub](https://img.shields.io/badge/DockerHub-webdevops%2Fpagerduty2es-blue)](https://hub.docker.com/r/webdevops/pagerduty2es/)
[![Quay.io](https://img.shields.io/badge/Quay.io-webdevops%2Fpagerduty2es-blue)](https://quay.io/repository/webdevops/pagerduty2es)

Exporter for incidents and logentries from PagerDuty to ElasticSearch

Configuration
-------------

```
Usage:
  pagerduty2es [OPTIONS]

Application Options:
      --debug                      debug mode [$DEBUG]
  -v, --verbose                    verbose mode [$VERBOSE]
      --log.json                   Switch log output to json format [$LOG_JSON]
      --pagerduty.authtoken=       PagerDuty auth token [$PAGERDUTY_AUTH_TOKEN]
      --pagerduty.date-range=      PagerDuty date range (default: 168h) [$PAGERDUTY_DATE_RANGE]
      --pagerduty.max-connections= Maximum numbers of TCP connections to PagerDuty API (concurrency) (default: 4)
                                   [$PAGERDUTY_MAX_CONNECTIONS]
      --elasticsearch.address=     ElasticSearch urls [$ELASTICSEARCH_ADDRESS]
      --elasticsearch.username=    ElasticSearch username for HTTP Basic Authentication [$ELASTICSEARCH_USERNAME]
      --elasticsearch.password=    ElasticSearch password for HTTP Basic Authentication [$ELASTICSEARCH_PASSWORD]
      --elasticsearch.apikey=      ElasticSearch base64-encoded token for authorization; if set, overrides username and
                                   password [$ELASTICSEARCH_APIKEY]
      --elasticsearch.index=       ElasticSearch index name (placeholders: %y for year, %m for month and %d for day)
                                   (default: pagerduty) [$ELASTICSEARCH_INDEX]
      --elasticsearch.batch-count= Number of documents which should be indexed in one request (default: 50)
                                   [$ELASTICSEARCH_BATCH_COUNT]
      --elasticsearch.retry-count= ElasticSearch request retry count (default: 5) [$ELASTICSEARCH_RETRY_COUNT]
      --elasticsearch.retry-delay= ElasticSearch request delay for reach retry (default: 5s)
                                   [$ELASTICSEARCH_RETRY_DELAY]
      --bind=                      Server address (default: :8080) [$SERVER_BIND]
      --scrape-time=               Scrape time (time.duration) (default: 5m) [$SCRAPE_TIME]

Help Options:
  -h, --help                       Show this help message
```

Metrics
-------

| Metric                                       | Description                                                        |
|----------------------------------------------|--------------------------------------------------------------------|
| `pagerduty2es_incident_total`                | Total number of processed incidents                                |
| `pagerduty2es_incident_logentry_total`       | Total number of processed logentries                               |
| `pagerduty2es_duration`                      | Scrape process duration                                            |
| `pagerduty2es_elasticsearch_requet_total`    | Number of total requests to ElasticSearch cluster                  |
| `pagerduty2es_elasticsearch_request_retries` | Number of retried requests to ElasticSearch cluster                |
