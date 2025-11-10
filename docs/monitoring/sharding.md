# Prometheus Agent Sharding

To be able to ingest metrics without disrupting the workload running in the clusters, the observability operator chooses the number of running __prometheus agent shards__ on each workload cluster. The number of shards is based on the __total number of time series__ ingested for a given cluster.

__By default__, the operator configures 1 shard for every 1M time series present in Mimir for the workload cluster. To avoid scaling down too abruptly, we defined a scale down threshold of 20%.

Scale up series threshold and scale down percentage are overridables.

1. Those values can be configured at the installation level by overriding the following values:

```yaml
monitoring:
  sharding:
    scaleUpSeriesCount: 1000000
    scaleDownPercentage: 0.20
```

2. Those values can also be set per cluster using the following cluster annotations:

```yaml
observability.giantswarm.io/monitoring-agent-scale-up-series-count: 1000000
observability.giantswarm.io/monitoring-agent-scale-down-percentage: 0.20
```
