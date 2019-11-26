/****************************************************************
* Pgbouncer Exporter
* Author:  Vonng(fengruohang@outlook.com)
* Created: 2019-11-26
* License: BSD
****************************************************************/
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"database/sql"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Version 0.0.1
var Version = "0.0.1"

var (
	listenAddress  string
	metricPath     string
	dataSourceName string
)

// Exporter collect pgbouncer metrics, and implement prometheus.Collector
type Exporter struct {
	DB   *sql.DB
	Desc map[string]*prometheus.Desc
	dsn  string
	rw   sync.Mutex

	// internal state
	pgbouncerUp    bool
	scrapeDuration time.Duration
	lastScrape     time.Time
	totalScrapes   int64
	errorCount     int64
}

// NewExporter returns a pgbouncer exporter for given DSN
func NewExporter(dsn string) (e *Exporter) {
	return &Exporter{dsn: dsn}
}

// Connect issue a connection to pgbouncer using dsn
func (e *Exporter) Connect() (err error) {
	e.DB, err = sql.Open("postgres", e.dsn)
	if err != nil {
		return errors.New(fmt.Sprintln("fail to connect to pgbouncer: ", err))
	}
	e.DB.SetMaxIdleConns(1)
	e.DB.SetMaxOpenConns(1)
	if err = e.DB.Ping(); err != nil {
		return errors.New(fmt.Sprintln("ping server failed: ", err))
	}
	e.pgbouncerUp = true
	return
}

// Close disconnect from pgbouncer
func (e *Exporter) Close() {
	e.rw.Lock()
	defer e.rw.Unlock()
	e.DB.Close()
}

// RegisterDescriptors will add prometheus descriptor to Exporter map
func (e *Exporter) RegisterDescriptors() {
	e.rw.Lock()
	defer e.rw.Unlock()
	e.Desc = make(map[string]*prometheus.Desc, 20)

	// Internal metrics
	e.Desc["pgbouncer_up"] = prometheus.NewDesc("pgbouncer_up", "whether pgbouncer is alive", nil, nil)
	e.Desc["pgbouncer_scrape_duration"] = prometheus.NewDesc("pgbouncer_scrape_duration", "time that spending on scrapping, in nanoseconds", nil, nil)
	e.Desc["pgbouncer_scrape_last_time"] = prometheus.NewDesc("pgbouncer_scrape_last_time", "last timestamp of scrape in unix epoch", nil, nil)
	e.Desc["pgbouncer_scrape_total"] = prometheus.NewDesc("pgbouncer_scrape_total", "total scrape count", nil, nil)
	e.Desc["pgbouncer_scrape_error_count"] = prometheus.NewDesc("pgbouncer_scrape_error_count", "total error count when scrapping", nil, nil)

	// List Descriptor
	e.Desc["pgbouncer_databases"] = prometheus.NewDesc("pgbouncer_databases", "pgbouncer total database count", nil, nil)
	e.Desc["pgbouncer_users"] = prometheus.NewDesc("pgbouncer_users", "pgbouncer total users count", nil, nil)
	e.Desc["pgbouncer_pools"] = prometheus.NewDesc("pgbouncer_pools", "pgbouncer total pools count", nil, nil)
	e.Desc["pgbouncer_free_clients"] = prometheus.NewDesc("pgbouncer_free_clients", "pgbouncer available clients count", nil, nil)
	e.Desc["pgbouncer_used_clients"] = prometheus.NewDesc("pgbouncer_used_clients", "pgbouncer used clients count", nil, nil)
	e.Desc["pgbouncer_login_clients"] = prometheus.NewDesc("pgbouncer_login_clients", "pgbouncer login clients count", nil, nil)
	e.Desc["pgbouncer_free_servers"] = prometheus.NewDesc("pgbouncer_free_servers", "pgbouncer available servers count", nil, nil)
	e.Desc["pgbouncer_used_servers"] = prometheus.NewDesc("pgbouncer_used_servers", "pgbouncer used servers count", nil, nil)
	e.Desc["pgbouncer_dns_names"] = prometheus.NewDesc("pgbouncer_dns_names", "pgbouncer dns name count", nil, nil)
	e.Desc["pgbouncer_dns_zones"] = prometheus.NewDesc("pgbouncer_dns_zones", "pgbouncer dns zone count", nil, nil)
	e.Desc["pgbouncer_dns_queries"] = prometheus.NewDesc("pgbouncer_dns_queries", "pgbouncer dns queries count", nil, nil)
	e.Desc["pgbouncer_dns_pending"] = prometheus.NewDesc("pgbouncer_dns_pending", "pgbouncer dns pending queries count", nil, nil)

	// Mem Descriptor
	e.Desc["pgbouncer_memory_usage"] = prometheus.NewDesc("pgbouncer_memory_usage", "pgbouncer memory usage", []string{"type"}, nil)

	// Stats Descriptor
	e.Desc["pgbouncer_stat_total_xact_count"] = prometheus.NewDesc("pgbouncer_stat_total_xact_count", "pgbouncer total_xact_count of show stats", []string{"datname"}, nil)
	e.Desc["pgbouncer_stat_total_query_count"] = prometheus.NewDesc("pgbouncer_stat_total_query_count", "pgbouncer total_query_count of show stats", []string{"datname"}, nil)
	e.Desc["pgbouncer_stat_total_received"] = prometheus.NewDesc("pgbouncer_stat_total_received", "pgbouncer total_received of show stats", []string{"datname"}, nil)
	e.Desc["pgbouncer_stat_total_sent"] = prometheus.NewDesc("pgbouncer_stat_total_sent", "pgbouncer total_sent of show stats", []string{"datname"}, nil)
	e.Desc["pgbouncer_stat_total_xact_time"] = prometheus.NewDesc("pgbouncer_stat_total_xact_time", "pgbouncer total_xact_time of show stats", []string{"datname"}, nil)
	e.Desc["pgbouncer_stat_total_query_time"] = prometheus.NewDesc("pgbouncer_stat_total_query_time", "pgbouncer total_query_time of show stats", []string{"datname"}, nil)
	e.Desc["pgbouncer_stat_total_wait_time"] = prometheus.NewDesc("pgbouncer_stat_total_wait_time", "pgbouncer total_wait_time of show stats", []string{"datname"}, nil)
	e.Desc["pgbouncer_stat_avg_xact_count"] = prometheus.NewDesc("pgbouncer_stat_avg_xact_count", "pgbouncer avg_xact_count of show stats", []string{"datname"}, nil)
	e.Desc["pgbouncer_stat_avg_query_count"] = prometheus.NewDesc("pgbouncer_stat_avg_query_count", "pgbouncer avg_query_count of show stats", []string{"datname"}, nil)
	e.Desc["pgbouncer_stat_avg_recv"] = prometheus.NewDesc("pgbouncer_stat_avg_recv", "pgbouncer avg_recv of show stats", []string{"datname"}, nil)
	e.Desc["pgbouncer_stat_avg_sent"] = prometheus.NewDesc("pgbouncer_stat_avg_sent", "pgbouncer avg_sent of show stats", []string{"datname"}, nil)
	e.Desc["pgbouncer_stat_avg_xact_time"] = prometheus.NewDesc("pgbouncer_stat_avg_xact_time", "pgbouncer avg_xact_time of show stats", []string{"datname"}, nil)
	e.Desc["pgbouncer_stat_avg_query_time"] = prometheus.NewDesc("pgbouncer_stat_avg_query_time", "pgbouncer avg_query_time of show stats", []string{"datname"}, nil)
	e.Desc["pgbouncer_stat_avg_wait_time"] = prometheus.NewDesc("pgbouncer_stat_avg_wait_time", "pgbouncer avg_wait_time of show stats", []string{"datname"}, nil)

	// Database Descriptor
	e.Desc["pgbouncer_database_pool_size"] = prometheus.NewDesc("pgbouncer_database_pool_size", "pgbouncer database pool_size from show databases", []string{"datname"}, nil)
	e.Desc["pgbouncer_database_reserve_pool"] = prometheus.NewDesc("pgbouncer_database_reserve_pool", "pgbouncer database reserve_pool from show databases", []string{"datname"}, nil)
	e.Desc["pgbouncer_database_max_connections"] = prometheus.NewDesc("pgbouncer_database_max_connections", "pgbouncer database max_connections from show databases", []string{"datname"}, nil)
	e.Desc["pgbouncer_database_current_connections"] = prometheus.NewDesc("pgbouncer_database_current_connections", "pgbouncer database current_connections from show databases", []string{"datname"}, nil)
	e.Desc["pgbouncer_database_paused"] = prometheus.NewDesc("pgbouncer_database_paused", "pgbouncer database paused from show databases", []string{"datname"}, nil)
	e.Desc["pgbouncer_database_disabled"] = prometheus.NewDesc("pgbouncer_database_disabled", "pgbouncer database disabled from show databases", []string{"datname"}, nil)

	// Pool Descriptor
	e.Desc["pgbouncer_pool_cl_active"] = prometheus.NewDesc("pgbouncer_pool_cl_active", "pgbouncer pool cl_active from show pools", []string{"datname", "user"}, nil)
	e.Desc["pgbouncer_pool_cl_waiting"] = prometheus.NewDesc("pgbouncer_pool_cl_waiting", "pgbouncer pool cl_waiting from show pools", []string{"datname", "user"}, nil)
	e.Desc["pgbouncer_pool_sv_active"] = prometheus.NewDesc("pgbouncer_pool_sv_active", "pgbouncer pool sv_active from show pools", []string{"datname", "user"}, nil)
	e.Desc["pgbouncer_pool_sv_idle"] = prometheus.NewDesc("pgbouncer_pool_sv_idle", "pgbouncer pool sv_idle from show pools", []string{"datname", "user"}, nil)
	e.Desc["pgbouncer_pool_sv_used"] = prometheus.NewDesc("pgbouncer_pool_sv_used", "pgbouncer pool sv_used from show pools", []string{"datname", "user"}, nil)
	e.Desc["pgbouncer_pool_sv_tested"] = prometheus.NewDesc("pgbouncer_pool_sv_tested", "pgbouncer pool sv_tested from show pools", []string{"datname", "user"}, nil)
	e.Desc["pgbouncer_pool_sv_login"] = prometheus.NewDesc("pgbouncer_pool_sv_login", "pgbouncer pool sv_login from show pools", []string{"datname", "user"}, nil)
	e.Desc["pgbouncer_pool_maxwait"] = prometheus.NewDesc("pgbouncer_pool_maxwait", "pgbouncer pool maxwait from show pools", []string{"datname", "user"}, nil)
	e.Desc["pgbouncer_pool_maxwait_us"] = prometheus.NewDesc("pgbouncer_pool_maxwait_us", "pgbouncer pool maxwait_us from show pools", []string{"datname", "user"}, nil)

}

// Collect implment prometheus.Collector
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.Scrape(ch)
}

// Describe implment prometheus.Collector
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, v := range e.Desc {
		ch <- v
	}
}

// Scrape issues query command to pgbouncer and produce metrics
func (e *Exporter) Scrape(ch chan<- prometheus.Metric) (err error) {
	e.rw.Lock()
	defer e.rw.Unlock()
	startTime := time.Now()
	if err = e.scrapeShowLists(ch); err != nil {
		goto final
	}
	if err = e.scrapeShowMem(ch); err != nil {
		goto final
	}
	if err = e.scrapeShowStats(ch); err != nil {
		goto final
	}
	if err = e.scrapeShowDatabases(ch); err != nil {
		goto final
	}
	if err = e.scrapeShowPools(ch); err != nil {
		goto final
	}

final:
	e.lastScrape = time.Now()
	e.scrapeDuration = e.lastScrape.Sub(startTime)
	e.totalScrapes++

	if err != nil {
		e.pgbouncerUp = false
		e.errorCount++
		log.Printf("scrape failed: %s", err.Error())
	} else {
		e.pgbouncerUp = true
	}

	// send internal metrics
	ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_up"], prometheus.GaugeValue, cast2Float64(e.pgbouncerUp))
	ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_scrape_duration"], prometheus.GaugeValue, cast2Float64(e.scrapeDuration))
	ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_scrape_last_time"], prometheus.GaugeValue, cast2Float64(e.lastScrape))
	ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_scrape_total"], prometheus.CounterValue, cast2Float64(e.totalScrapes))
	ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_scrape_error_count"], prometheus.CounterValue, cast2Float64(e.errorCount))

	return err
}

// scrapeShowLists fetch metrics from `SHOW LISTS`
func (e *Exporter) scrapeShowLists(ch chan<- prometheus.Metric) (err error) {
	rows, err := e.DB.Query(`SHOW LISTS;`)
	if err != nil {
		return errors.New(fmt.Sprintln("Error retrieving rows: ", err))
	}
	defer rows.Close()

	nColumn := 2
	columnData := make([]interface{}, nColumn)
	scanArgs := make([]interface{}, nColumn)
	for i := 0; i < nColumn; i++ {
		scanArgs[i] = &columnData[i]
	}

	listResult := make(map[string]float64)
	for rows.Next() {
		if err = rows.Scan(scanArgs...); err != nil {
			fmt.Println(err.Error())
			return errors.New(fmt.Sprintln("Error scanning rows: ", err))
		}

		listResult[cast2string(columnData[0])] = cast2Float64(columnData[1])
	}

	for k, v := range listResult {
		ch <- prometheus.MustNewConstMetric(e.Desc[fmt.Sprintf("pgbouncer_%s", k)], prometheus.GaugeValue, v)
	}

	return nil
}

// scrapeShowMem fetch metrics from `SHOW MEM`
func (e *Exporter) scrapeShowMem(ch chan<- prometheus.Metric) (err error) {
	rows, err := e.DB.Query(`SHOW MEM;`)
	if err != nil {
		return errors.New(fmt.Sprintln("Error retrieving rows: ", err))
	}
	defer rows.Close()

	nColumn := 5
	columnData := make([]interface{}, nColumn)
	scanArgs := make([]interface{}, nColumn)
	for i := 0; i < nColumn; i++ {
		scanArgs[i] = &columnData[i]
	}

	memResult := make(map[string]float64)
	for rows.Next() {
		if err = rows.Scan(scanArgs...); err != nil {
			fmt.Println(err.Error())
			return errors.New(fmt.Sprintln("Error scanning rows: ", err))
		}
		memResult[cast2string(columnData[0])] = cast2Float64(columnData[4])
	}

	for k, v := range memResult {
		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_memory_usage"], prometheus.GaugeValue, v, k)
	}
	return nil
}

// scrapeShowStats fetch metrics from `SHOW STATS`
func (e *Exporter) scrapeShowStats(ch chan<- prometheus.Metric) (err error) {
	rows, err := e.DB.Query(`SHOW STATS;`)
	if err != nil {
		return errors.New(fmt.Sprintln("Error retrieving rows: ", err))
	}
	defer rows.Close()

	nColumn := 15
	columnData := make([]interface{}, nColumn)
	scanArgs := make([]interface{}, nColumn)
	for i := 0; i < nColumn; i++ {
		scanArgs[i] = &columnData[i]
	}

	statResult := make(map[string]map[string]float64, 5)
	for rows.Next() {
		if err = rows.Scan(scanArgs...); err != nil {
			fmt.Println(err.Error())
			return errors.New(fmt.Sprintln("Error scanning rows: ", err))
		}

		statRow := make(map[string]float64, 14)
		datname := cast2string(columnData[0])
		statRow["total_xact_count"] = cast2Float64(columnData[1])
		statRow["total_query_count"] = cast2Float64(columnData[2])
		statRow["total_received"] = cast2Float64(columnData[3])
		statRow["total_sent"] = cast2Float64(columnData[4])
		statRow["total_xact_time"] = cast2Float64(columnData[5])
		statRow["total_query_time"] = cast2Float64(columnData[6])
		statRow["total_wait_time"] = cast2Float64(columnData[7])
		statRow["avg_xact_count"] = cast2Float64(columnData[8])
		statRow["avg_query_count"] = cast2Float64(columnData[9])
		statRow["avg_recv"] = cast2Float64(columnData[10])
		statRow["avg_sent"] = cast2Float64(columnData[11])
		statRow["avg_xact_time"] = cast2Float64(columnData[12])
		statRow["avg_query_time"] = cast2Float64(columnData[13])
		statRow["avg_wait_time"] = cast2Float64(columnData[14])
		statResult[datname] = statRow
	}

	for datname, datStat := range statResult {
		for k, v := range datStat {
			if strings.HasPrefix(k, "total") {
				ch <- prometheus.MustNewConstMetric(e.Desc[fmt.Sprintf("pgbouncer_stat_%s", k)], prometheus.CounterValue, v, datname)
			} else {
				ch <- prometheus.MustNewConstMetric(e.Desc[fmt.Sprintf("pgbouncer_stat_%s", k)], prometheus.GaugeValue, v, datname)
			}
		}
	}

	return nil
}

// scrapeShowDatabases fetch metrics from `SHOW DATABASES`
func (e *Exporter) scrapeShowDatabases(ch chan<- prometheus.Metric) (err error) {
	rows, err := e.DB.Query(`SHOW DATABASES;`)
	if err != nil {
		return errors.New(fmt.Sprintln("Error retrieving rows: ", err))
	}
	defer rows.Close()

	nColumn := 12
	columnData := make([]interface{}, nColumn)
	scanArgs := make([]interface{}, nColumn)
	for i := 0; i < nColumn; i++ {
		scanArgs[i] = &columnData[i]
	}

	for rows.Next() {
		if err = rows.Scan(scanArgs...); err != nil {
			fmt.Println(err.Error())
			return errors.New(fmt.Sprintln("Error scanning rows: ", err))
		}

		datname := cast2string(columnData[0])
		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_database_pool_size"], prometheus.GaugeValue, cast2Float64(columnData[5]), datname)
		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_database_reserve_pool"], prometheus.GaugeValue, cast2Float64(columnData[6]), datname)
		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_database_max_connections"], prometheus.GaugeValue, cast2Float64(columnData[8]), datname)
		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_database_current_connections"], prometheus.GaugeValue, cast2Float64(columnData[9]), datname)
		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_database_paused"], prometheus.GaugeValue, cast2Float64(columnData[10]), datname)
		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_database_disabled"], prometheus.GaugeValue, cast2Float64(columnData[11]), datname)

	}
	return nil
}

// scrapeShowPools fetch metrics from `SHOW POOLS`
func (e *Exporter) scrapeShowPools(ch chan<- prometheus.Metric) (err error) {
	rows, err := e.DB.Query(`SHOW POOLS;`)
	if err != nil {
		return errors.New(fmt.Sprintln("Error retrieving rows: ", err))
	}
	defer rows.Close()

	nColumn := 12
	columnData := make([]interface{}, nColumn)
	scanArgs := make([]interface{}, nColumn)
	for i := 0; i < nColumn; i++ {
		scanArgs[i] = &columnData[i]
	}

	for rows.Next() {
		if err = rows.Scan(scanArgs...); err != nil {
			fmt.Println(err.Error())
			return errors.New(fmt.Sprintln("Error scanning rows: ", err))
		}

		datname := cast2string(columnData[0])
		username := cast2string(columnData[1])

		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_pool_cl_active"], prometheus.GaugeValue, cast2Float64(columnData[2]), datname, username)
		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_pool_cl_waiting"], prometheus.GaugeValue, cast2Float64(columnData[3]), datname, username)
		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_pool_sv_active"], prometheus.GaugeValue, cast2Float64(columnData[4]), datname, username)
		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_pool_sv_idle"], prometheus.GaugeValue, cast2Float64(columnData[5]), datname, username)
		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_pool_sv_used"], prometheus.GaugeValue, cast2Float64(columnData[6]), datname, username)
		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_pool_sv_tested"], prometheus.GaugeValue, cast2Float64(columnData[7]), datname, username)
		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_pool_sv_login"], prometheus.GaugeValue, cast2Float64(columnData[8]), datname, username)
		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_pool_maxwait"], prometheus.GaugeValue, cast2Float64(columnData[9]), datname, username)
		ch <- prometheus.MustNewConstMetric(e.Desc["pgbouncer_pool_maxwait_us"], prometheus.GaugeValue, cast2Float64(columnData[10]), datname, username)
	}
	return nil
}

// cast2Float64 cast database driver interface{} to float64
func cast2Float64(t interface{}) float64 {
	switch v := t.(type) {
	case int64:
		return float64(v)
	case float64:
		return v
	case time.Time:
		return float64(v.Unix())
	case time.Duration:
		return float64(v.Nanoseconds())
	case []byte:
		strV := string(v)
		result, err := strconv.ParseFloat(strV, 64)
		if err != nil {
			return math.NaN()
		}
		return result
	case string:
		result, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return math.NaN()
		}
		return result
	case bool:
		if v {
			return 1.0
		}
		return 0.0
	case nil:
		return math.NaN()
	default:
		return math.NaN()
	}
}

// cast2Float64 cast database driver interface{} to string
func cast2string(t interface{}) string {
	switch v := t.(type) {
	case int64:
		return fmt.Sprintf("%v", v)
	case float64:
		return fmt.Sprintf("%v", v)
	case time.Time:
		return fmt.Sprintf("%v", v.Unix())
	case nil:
		return ""
	case []byte:
		return string(v)
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

// ParseEnv will parse environment variable into switch variable (override arguments)
func ParseEnv() {
	if dsn := os.Getenv("DATA_SOURCE_NAME"); len(dsn) != 0 {
		dataSourceName = dsn
	}
	if la := os.Getenv("PGB_EXPORTER_WEB_LISTEN_ADDRESS"); len(la) != 0 {
		listenAddress = la
	}
	if wtp := os.Getenv("PGB_EXPORTER_WEB_TELEMETRY_PATH"); len(wtp) != 0 {
		metricPath = wtp
	}
}

func main() {
	// parse arguements
	flag.StringVar(&listenAddress, "l", ":9186", "Address to listen on for web interface and telemetry")
	flag.StringVar(&metricPath, "p", "/debug/metrics", "url path under which to expose metrics")
	flag.StringVar(&dataSourceName, "d", "host=/tmp port=6432 user=pgbouncer dbname=pgbouncer sslmode=disabled", "pgbouncer dsn/url in postgres format")
	flag.Parse()
	ParseEnv()

	// Create new exporter
	exporter := NewExporter(dataSourceName)
	if err := exporter.Connect(); err != nil {
		log.Printf("Fail to connect to pgbouncer, waiting... : %s", err.Error())
	}
	defer exporter.Close()

	// Register prometheus descriptors
	exporter.RegisterDescriptors()
	prometheus.MustRegister(exporter)
	http.Handle(metricPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.Write([]byte(`<html><head><title>Pgbouncer Exporter</title></head><body><h1>Pgbouncer Exporter</h1><p><a href='` + metricPath + `'>Metrics</a></p></body></html>`))
	})

	log.Printf("Starting Server: %s%s", listenAddress, metricPath)
	log.Fatal(http.ListenAndServe(listenAddress, nil))
}
