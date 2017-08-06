# Bosh System Metrics Forwarder Release

This consumes bosh health events/metrics and forwards them to Loggregator.

## Architecture

![architecture dig][diagram]

Upon startup, the forwarder obtains a token from the UAA on the director using client credentials. The forwarder uses this token to connect to the [Bosh System Metrics Server][server] which verifies that the token contains the `bosh.system_metrics.read` authority. 

Once verified, the server begins streaming metrics via secure grpc to the forwarder, which then translates it to loggregator envelopes and sends them to metron via secure grpc. 

[server]: https://github.com/pivotal-cf/bosh-system-metrics-server-release
[diagram]: https://docs.google.com/a/pivotal.io/drawings/d/1l1iAQaBc6SHIpWb3x-lI9p4JVIZN_3ErepbAohqnaPw/pub?w=1192&h=719
