# Alloy Queue Configuration

The observability operator supports configuring all Alloy remote write queue parameters through helm values. This allows you to tune the queue behavior based on your specific requirements.

## Configuration

### Helm Values

All queue configuration fields are optional. When not specified, Alloy uses its default values. Configure them in your helm values:

```yaml
monitoring:
  queueConfig:
    # Maximum time samples wait in the buffer before sending (default: "5s")
    batchSendDeadline: "10s"
    
    # Number of samples to buffer per shard (default: 10000)
    capacity: 15000
    
    # Maximum retry delay (default: "5s")
    maxBackoff: "10s"
    
    # Maximum number of samples per send (default: 2000)
    maxSamplesPerSend: 5000
    
    # Maximum number of concurrent shards (default: 50)
    maxShards: 100
    
    # Initial retry delay (default: "30ms")
    minBackoff: "50ms"
    
    # Minimum number of concurrent shards (default: 1)
    minShards: 2
    
    # Retry when an HTTP 429 status code is received (default: true)
    retryOnHttp429: false
    
    # Maximum age of samples to send (default: "0s" - disabled)
    sampleAgeLimit: "1h"
```

### Command Line Flags

The queue configuration can also be set via command line flags:

- `--monitoring-queue-config-batch-send-deadline`
- `--monitoring-queue-config-capacity`
- `--monitoring-queue-config-max-backoff`
- `--monitoring-queue-config-max-samples-per-send`
- `--monitoring-queue-config-max-shards`
- `--monitoring-queue-config-min-backoff`
- `--monitoring-queue-config-min-shards`
- `--monitoring-queue-config-retry-on-http-429`
- `--monitoring-queue-config-sample-age-limit`

## Queue Configuration Parameters

For detailed information about each queue configuration parameter, see the [Grafana Alloy prometheus.remote_write queue_config documentation](https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.remote_write/#queue_config).

## Tuning Guidelines

### High Throughput Environments
- Increase `maxShards` and `maxSamplesPerSend`
- Consider increasing `capacity` for better batching
- Monitor queue depth and adjust accordingly

### Low Latency Requirements
- Decrease `batchSendDeadline`
- Consider increasing `minShards` for more parallelism

### Memory Constrained Environments
- Decrease `capacity` and `maxShards`
- Monitor memory usage and queue performance

### Unreliable Networks
- Increase `maxBackoff` and enable `retryOnHttp429`
- Consider shorter `sampleAgeLimit` to avoid accumulating stale data

## Monitoring

Monitor these metrics to understand queue behavior. These are visible on the Grafana dashboard `Prometheus / Remote Write` with ID `promRW001/prometheus-remote-write`:

- `prometheus_remote_storage_shards`: Current number of shards
- `prometheus_remote_storage_shards_desired`: Desired number of shards
- `prometheus_remote_storage_samples_pending`: Samples pending in queues
- `prometheus_remote_storage_shard_capacity`: Capacity of shards

## References

- [Grafana Alloy prometheus.remote_write Documentation](https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.remote_write/#queue_config)
