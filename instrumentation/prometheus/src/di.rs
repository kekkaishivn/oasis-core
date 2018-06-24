//! Dependency injection support.
use super::{pusher, server, PrometheusMetricCollector};
use ekiden_instrumentation::MetricCollector;
use std::net::SocketAddr;
use std::time::Duration;

const PROMETHEUS_MODE_PULL: &'static str = "pull";
const PROMETHEUS_MODE_PUSH: &'static str = "push";

// Make Prometheus service injectable.
create_component!(
    prometheus,
    "metric-collector",
    PrometheusMetricCollector,
    MetricCollector,
    (|container: &mut Container| -> Result<Box<Any>> {
        // Start Prometheus metrics endpoint or pusher when available.
        #[cfg(any(feature = "server", feature = "pusher"))]
        {
            let environment = container.inject()?;
            let args = container.get_arguments().unwrap();
            let mode = value_t!(args, "prometheus-mode", String);
            match mode.as_ref().map(|x| x.as_ref()) {
                Ok(PROMETHEUS_MODE_PULL) => {
                    if let Ok(address) = value_t!(args, "metrics-addr", SocketAddr) {
                        server::start(environment, address);
                    }
                }
                Ok(PROMETHEUS_MODE_PUSH) => {
                    if let Ok(address) = value_t!(args, "metrics-addr", String) {
                        let period = value_t!(args, "push-period", u64).unwrap_or(5);
                        pusher::start(environment, address, Duration::from_secs(period));
                    }
                }
                _ => (),
            }
        }
        let metric_collector: Box<MetricCollector> = Box::new(PrometheusMetricCollector::new());
        Ok(Box::new(metric_collector))
    }),
    [
        Arg::with_name("prometheus-mode")
            .long("prometheus-mode")
            .help("Prometheus mode to be used: pull/push")
            .possible_values(&[PROMETHEUS_MODE_PULL, PROMETHEUS_MODE_PUSH])
            .default_value(PROMETHEUS_MODE_PULL)
            .takes_value(true),
        Arg::with_name("push-period")
            .long("push-period")
            .help("Push period in seconds, if using 'push' mode.")
            .takes_value(true),
        Arg::with_name("metrics-addr")
            .long("metrics-addr")
            .help("If pull mode: A SocketAddr (as a string) from which to serve metrics to Prometheus. If push mode: prometheus 'pushgateway' address.")
            .takes_value(true)
    ]
);
