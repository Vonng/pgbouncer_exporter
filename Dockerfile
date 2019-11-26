FROM scratch
ADD pgbouncer_exporter /
CMD ["/pgbouncer_exporter"]
