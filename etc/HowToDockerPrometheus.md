Run Prometheus with config from `etc` (assuming that you're in `anycable-go` source directory):

```
docker run -p 9090:9090 -v $(pwd)/etc:/prometheus-data prom/prometheus --config.file=/prometheus-data/prometheus.yml
```

Run Grafana:

```
docker run --name=grafana -p 3100:3000 grafana/grafana
```

And, finally, run `anycable-go` (locally):

```
anycable-go --metrics_http="/metrics"
```
