## Change Log

#### dcgm-exporter

> Note that in the next releases, metric labels may change.

* 1.0.0-beta
	* Semantic Versioning: Currently dcgm-exporter is versioned according to the DCGM releases. Starting from dcgm-exporter:1.0.0, dcgm-exporter will have its own semantic versioning.
	* dcgm-exporter:1.4.6 based on DCGM 1.4.6 will be updated to dcgm-exporter:1.0.0 based on DCGM 1.7.1.
	* DCP metrics: We have added new [DCGM Data Center Profiling (DCP)](https://docs.nvidia.com/datacenter/dcgm/1.7/dcgm-user-guide/feature-overview.html#profiling) metrics but these metrics will not be collected by default. To collect DCP metrics, you will need to pass "-p" option to the dcgm-exporter script.

