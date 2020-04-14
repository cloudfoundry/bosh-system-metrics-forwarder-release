## DEPRECATED
This repo is being maintained but there is no active development.
Please refer to the [system-metrics-release](https://github.com/cloudfoundry/system-metrics-release) 
for the newer architecture for getting system metrics.

# Bosh System Metrics Forwarder Release

This consumes bosh health events and forwards _heartbeat events only_ to Loggregator. For more info, see https://github.com/cloudfoundry/bosh-system-metrics-server-release/wiki

## Architecture

![architecture dig][diagram]

The forwarder obtains a token from the UAA on the director using client credentials before establishing the connection to the [Bosh System Metrics Server][server]. The server verifies that the token contains the `bosh.system_metrics.read` authority.

Once verified, the server begins streaming events via secure grpc to the forwarder. Currently, the forwarder ignores alerts and translates the heartbeat events to loggregator envelopes and sends them to metron via secure grpc.

[server]: https://github.com/cloudfoundry/bosh-system-metrics-server-release
[diagram]: https://docs.google.com/a/pivotal.io/drawings/d/1l1iAQaBc6SHIpWb3x-lI9p4JVIZN_3ErepbAohqnaPw/pub?w=1192&h=719

## Development setup

Dependencies are managed with submodules. Run `git submodule update --recursive` to install.
