# Scout Grafana Dashboard

Import `scout-dashboard.json` into Grafana to monitor Scout metrics.

## Prerequisites

- Scout running with `scout agent serve` (exposes /metrics)
- Prometheus scraping the /metrics endpoint
- Grafana with Prometheus datasource configured

## Import

1. In Grafana, go to Dashboards → Import
2. Upload `scout-dashboard.json`
3. Select your Prometheus datasource
