# Pgbouncer Exporter

Prometheus exporter for [Pgbouncer](https://www.pgbouncer.org) metrics

Tested on pgbouncer `1.12`, should be backward compatible to pgbouncer `1.8.x`

The latest version is `0.0.1`, you can donwnload binaries at the release page.



## Quick Start

You can do it the bare metal way:

```bash
DATA_SOURCE_NAME='host=/tmp port=6432 user=pgbouncer dbname=pgbouncer' ./pgbouncer_exporter
```

Or the docker way:

```bash
docker run \
  --env=DATA_SOURCE_NAME='host=docker.for.mac.host.internal port=6432 user=stats dbname=pgbouncer sslmode=disable' \
  --env=PGB_EXPORTER_WEB_LISTEN_ADDRESS=':9186' \
  --env=PGB_EXPORTER_WEB_TELEMETRY_PATH='/debug/metrics' \
  -p 9186:9186 \
  pgbouncer_exporter
```

The default listen address is `localhost:9186` and the default telemetry path is `/debug/metrics`. 

```bash
curl localhost:9186/debug/metrics
```

And the default data source name is:

```bash
host=/tmp port=6432 user=pgbouncer dbname=pgbouncer sslmode=disabled
```



## Build

```
go build
```

To build a static stand alone binary for docker scratch

```bash
CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o pgbouncer_exporter
```

To build a docker image, using:

```
docker build -t pgbouncer_exporter .
```



## Run

```bash
pgbouncer_exporter [-d <data source>] [-l <listen address>] [-p <telemetry path>]
```

There are three arguments: `data_source_name(-d)`, `listen_address(-l)`, `telemetry_path(-p)`

* `-d` controls the data source, maybe it is the only thing you need to change
* `-l` controls the listen address, `':9186` by default
* `-p` controls the telemetry path. `/debug/metrics` by default

The three arguments above can also be passed using environment variables. Environment variables will override command line arguments 

* `DATA_SOURCE_NAME` controls the data source.
  * `host=/tmp port=6432 user=pgbouncer dbname=pgbouncer sslmode=disabled` by default
  * Make sure you can connect to your pgbouncer using your `DATA_SOURCE_NAME`
* `PGB_EXPORTER_WEB_LISTEN_ADDRESS`  controls the listen address, `':9186` by default
* `PGB_EXPORTER_WEB_TELEMETRY_PATH` controls the telemetry path. `/debug/metrics` by default

Pgbouncer export will waiting for pgbouncer instead of fast failing during startup stage.



## Metrics

Metrics are scrapped from pgbouncer using admin commands: `SHOW LISTS`, `SHOW MEM`,`SHOW STATS`,`SHOW POOLS`, `SHOW DATABASES`.

```bash
# common metrics
pgbouncer_up
pgbouncer_scrape_duration
pgbouncer_scrape_last_time
pgbouncer_scrape_total
pgbouncer_scrape_error_count

# list metrics
pgbouncer_databases
pgbouncer_users
pgbouncer_pools
pgbouncer_free_clients
pgbouncer_used_clients
pgbouncer_login_clients
pgbouncer_free_servers
pgbouncer_used_servers
pgbouncer_dns_names
pgbouncer_dns_zones
pgbouncer_dns_queries
pgbouncer_dns_pending

# mem metrics
pgbouncer_memory_usage

# stats metrics
pgbouncer_stat_total_xact_count{datname}
pgbouncer_stat_total_query_count{datname}
pgbouncer_stat_total_received{datname}
pgbouncer_stat_total_sent{datname}
pgbouncer_stat_total_xact_time{datname}
pgbouncer_stat_total_query_time{datname}
pgbouncer_stat_total_wait_time{datname}
pgbouncer_stat_avg_xact_count{datname}
pgbouncer_stat_avg_query_count{datname}
pgbouncer_stat_avg_recv{datname}
pgbouncer_stat_avg_sent{datname}
pgbouncer_stat_avg_xact_time{datname}
pgbouncer_stat_avg_query_time{datname}
pgbouncer_stat_avg_wait_time{datname}

# database metrics
pgbouncer_database_pool_size{datname}
pgbouncer_database_reserve_pool{datname}
pgbouncer_database_max_connections{datname}
pgbouncer_database_current_connections{datname}
pgbouncer_database_paused{datname}
pgbouncer_database_disabled{datname}

# pool metrics
pgbouncer_pool_cl_active{datname,user}
pgbouncer_pool_cl_waiting{datname,user}
pgbouncer_pool_sv_active{datname,user}
pgbouncer_pool_sv_idle{datname,user}
pgbouncer_pool_sv_used{datname,user}
pgbouncer_pool_sv_tested{datname,user}
pgbouncer_pool_sv_login{datname,user}
pgbouncer_pool_maxwait{datname,user}
pgbouncer_pool_maxwait_us{datname,user}
```



## About

Author：Vonng ([fengruohang@outlook.com](mailto:fengruohang@outlook.com))

License：BSD