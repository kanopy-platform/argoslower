# argoslower
Ah go slower! is a controller for reacting to and mutating argo-events resources.
Its original purpose is to inject rate limits for sensors with k8s trigger
targets.

## Flags
- `rate-limit-unit-annotation` sets the namespace annotation key to look for the [RateLimit unit](https://github.com/argoproj/argo-events/blob/master/api/sensor.md#ratelimit) value
- `requests-per-unit-annotation` sets the namespace annotation key to look for the [RateLimit requestsPerUnit](https://github.com/argoproj/argo-events/blob/master/api/sensor.md#ratelimit) value
