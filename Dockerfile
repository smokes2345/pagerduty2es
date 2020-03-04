FROM golang:1.14 as build

WORKDIR /go/src/github.com/webdevops/pagerduty2elasticsearch-exporter

# Get deps (cached)
COPY ./go.mod /go/src/github.com/webdevops/pagerduty2elasticsearch-exporter
COPY ./go.sum /go/src/github.com/webdevops/pagerduty2elasticsearch-exporter
RUN go mod download

# Compile
COPY ./ /go/src/github.com/webdevops/pagerduty2elasticsearch-exporter
RUN make lint
RUN make build
RUN ./pagerduty-exporter --help

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/static
COPY --from=build /go/src/github.com/webdevops/pagerduty2elasticsearch-exporter/pagerduty2elasticsearch-exporter /
USER 1000
ENTRYPOINT ["/pagerduty2elasticsearch-exporter"]
