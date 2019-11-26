VERSION=`cat pgbouncer_exporter.go | grep -E 'var Version' | grep -Eo '[0-9.]+'`

build:
	go build -o pgbouncer_exporter

clean:
	rm -rf bin/pgbouncer_exporter

release-darwin: clean
	GOOS=darwin GOARCH=amd64 go build -o pgbouncer_exporter
	upx pgbouncer_exporter
	tar -cf bin/pgbouncer_exporter_v$(VERSION)_darwin-amd64.tar.gz pgbouncer_exporter
	rm -rf pgbouncer_exporter

release-linux: clean
	GOOS=linux GOARCH=amd64 go build -o pgbouncer_exporter
	upx pgbouncer_exporter
	tar -cf bin/pgbouncer_exporter_v$(VERSION)_linux-amd64.tar.gz pgbouncer_exporter
	rm -rf pgbouncer_exporter

release-windows: clean
	GOOS=windows GOARCH=amd64 go build -o pgbouncer_exporter
	upx pgbouncer_exporter
	tar -cf bin/pgbouncer_exporter_v$(VERSION)_windows-amd64.tar.gz pgbouncer_exporter
	rm -rf pgbouncer_exporter

docker: clean
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o pgbouncer_exporter
	docker build -t pgbouncer_exporter .

curl:
	curl localhost:9186/debug/metrics | grep pgbouncer

release: release-linux release-darwin release-windows

.PHONY: build clean release docker curl