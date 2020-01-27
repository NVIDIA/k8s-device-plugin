install:
	install -d -m 755 /run/prometheus
	install -m 775 ../dcgm-exporter/dcgm-exporter /usr/local/bin/
	install -m 644 prometheus-dcgm.service /etc/systemd/system/

uninstall:
	rm -rf /run/prometheus
	rm -f /usr/local/bin/dcgm-exporter
	rm -f /etc/systemd/system/prometheus-dcgm.service
