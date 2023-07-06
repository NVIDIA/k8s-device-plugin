## The GFD Command line interface

Available options:
```
gpu-feature-discovery:
Usage:
  gpu-feature-discovery [--fail-on-init-error=<bool>] [--mig-strategy=<strategy>] [--oneshot | --sleep-interval=<seconds>] [--no-timestamp] [--output-file=<file> | -o <file>]
  gpu-feature-discovery -h | --help
  gpu-feature-discovery --version

Options:
  -h --help                       Show this help message and exit
  --version                       Display version and exit
  --oneshot                       Label once and exit
  --no-timestamp                  Do not add timestamp to the labels
  --fail-on-init-error=<bool>     Fail if there is an error during initialization of any label sources [Default: true]
  --sleep-interval=<seconds>      Time to sleep between labeling [Default: 60s]
  --mig-strategy=<strategy>       Strategy to use for MIG-related labels [Default: none]
  -o <file> --output-file=<file>  Path to output file
                                  [Default: /etc/kubernetes/node-feature-discovery/features.d/gfd]

Arguments:
  <strategy>: none | single | mixed

```

You can also use environment variables:

| Env Variable           | Option               | Example |
| ---------------------- | -------------------- | ------- |
| GFD_FAIL_ON_INIT_ERROR | --fail-on-init-error | true    |
| GFD_MIG_STRATEGY       | --mig-strategy       | none    |
| GFD_ONESHOT            | --oneshot            | TRUE    |
| GFD_NO_TIMESTAMP       | --no-timestamp       | TRUE    |
| GFD_OUTPUT_FILE        | --output-file        | output  |
| GFD_SLEEP_INTERVAL     | --sleep-interval     | 10s     |

Environment variables override the command line options if they conflict.