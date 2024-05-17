# jsonmetrics

This program reads JSON from standard input, parses it for interesting values,
and exposes the results via HTTP in a text-based format compatible with
Prometheus. It might be useful for many purposes but I use it to feed Zigbee
sensor readings via MQTT to Prometheus to store them as time series.

## Disclaimer

The program is very much work-in-progress. I use it as it is for my own needs
but it will benefit from some polishing.

## Usage

When running, `jsonmetrics` exposes the metrics by default on
`http://localhost:19100/metrics`. A configuration file is the only required
command-line parameter:

    $ jsonmetrics -config metrics.ini

As `jsonmetrics` expects to be fed individual JSON values on each line, the
intended usage pattern is to pipe a data generator to its standard input:

    $ json_data_generator | jsonmetrics -config metrics.ini

When successfully running, you may then set something like Prometheus to scrape
the metrics endpoint or just `curl` it:

    $ curl http://localhost:19100/metrics

For all available options, see

    $ jsonmetrics -help

## Example configuration

Let us assume that the user wants to read metrics from JSON output that
[Zigbee2MQTT](https://www.zigbee2mqtt.io) generates and
[mosquitto_sub](https://mosquitto.org/man/mosquitto_sub-1.html) can print out.
It may look something like this when suitably mangled and pretty-printed:

```json
{
  "tst": "2024-04-28T09:55:17Z",
  "topic": "zigbee2mqtt/DEVICENAME",
  "qos": 0,
  "retain": 0,
  "payloadlen": 123,
  "payload": {
    "battery": 100,
    "sensor_value": 321,
    "sensor_boolean": true,
    "linkquality": 129
  }
}
```

If we want to extract and expose `sensor_value` and `sensor_boolean` for
`DEVICENAME` we may use the following configuration:

```ini
[metric.sensor_value_DEVICENAME]
metrictype=gauge
valuetype=int
source=zigbee2mqtt/DEVICENAME
sourcepath=.topic
rendername=sensor_value{dev="DEVICENAME"}
valuepath=.payload.sensor_value
timestamppath=.tst
timestampformat=2006-01-02T15:04:05Z07:00

[metric.sensor_boolean_DEVICENAME]
metrictype=gauge
valuetype=int
source=zigbee2mqtt/DEVICENAME
sourcepath=.topic
valuepath=.payload.sensor_boolean
rendername=sensor_boolean{dev="DEVICENAME"}
timestamppath=.tst
timestampformat=2006-01-02T15:04:05Z07:00
map=true:1
map=false:0
debounce=3m
```

The example sample above generates the following `/metrics` output:

```
sensor_boolean{dev="DEVICENAME"} 1 1714298117000
sensor_value{dev="DEVICENAME"} 321 1714298117000
```

## Transient values

Some sensors may quickly visit a certain state such as a `false` becoming `true`
only for a short moment. When exposing metrics such as these to Prometheus, the
scraping period is often much longer than the state duration. This means that
the scraper has a very low chance of catching those quick transitions.

For this reason, the metric configuration supports the `debounce` parameter
which is a time duration. It works such that when the metric's value changes,
`jsonmetrics` will maintain the new value until the `debounce` period has
passed. This gives the scraper a better chance to notice the new datax.

## Input format

The program will opportunistically parse individual lines expecting to find a
valid JSON parse. If your source outputs multiline JSON data, you may use
something like `jq` to condense it before feeding it to `jsonmetrics`:

```
$ echo '{
    "value": 1,
    "another": true
}' | jq -c
{"value":1,"another":true}
```

The extraction of JSON values is done with Kubernetes' Go client's
[jsonpath](https://pkg.go.dev/k8s.io/client-go/util/jsonpath) library. See the
library itself for support regarding more complex expressions.

## A realistic integration example: Zigbee sensor data via MQTT

Let us assume that the user has a MQTT broker which expects certificate-based
authentication. The user wants to listen for sensor values published on many
topics under the root of `zigbee2mqtt`. Additionally, the metrics should be
exposed via `http://localhost:19101/metrics`. The user has the relevant files
stored in `/opt/zigbee` and `jsonmetrics` is somewhere in `PATH`. The runner
script could then look something like:

```shell
#!/bin/sh
BROKER=broker.example.com
TOPIC=zigbee2mqtt/+ # subscribe with a single-level wildcard
PORT=1883
AUTH="--cert /opt/zigbee/zigbee.crt
      --key /opt/zigbee/zigbee.key
      --cafile /opt/zigbee/ca.crt"
TAG=zigbeemetrics
CLIENTID=zigbee-listener
export TZ=UTC # Needed for `fromdateiso8601` below.

echo "start $(date)"
mosquitto_sub \
    -h "$BROKER" \
    -p "$PORT" \
    -t "$TOPIC" \
    $AUTH \
    -i "$CLIENTID" \
    -F %j \
    | jq --unbuffered -Mc '
        .tst |= (.[:index(".")] + "Z" | fromdateiso8601 | todate)
        | .payload |= fromjson' \
    | jsonmetrics -config /opt/zigbee/metrics.ini -listen localhost:19101
```

The above may also be wrapped into a systemd unit file which may look something
like this:

```ini
[Unit]
Description=zigbeemetrics

[Service]
Type=simple
ExecStart=/bin/sh /opt/zigbee/zigbee.sh
Restart=always
RestartSec=10
User=runner
Group=runner

[Install]
WantedBy=multi-user.target
```
