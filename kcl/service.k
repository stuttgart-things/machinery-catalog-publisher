"""
Service for machinery-catalog-publisher — exposes the metrics/health port for
Prometheus scraping. No gRPC/HTMX service (the publisher serves no inbound API).
"""

import labels

config = labels.config

service = {
    apiVersion: "v1"
    kind: "Service"
    metadata: {
        name: config.name
        namespace: config.namespace
        labels: labels.commonLabels
        annotations: {
            "prometheus.io/scrape": "true"
            "prometheus.io/port": str(config.metricsPort)
            "prometheus.io/path": "/metrics"
        }
    }
    spec: {
        type: config.serviceType
        ports: [
            {
                name: "metrics"
                port: config.metricsPort
                targetPort: "metrics"
                protocol: "TCP"
            }
        ]
        selector: labels.selectorLabels
    }
} if config.serviceEnabled else Undefined
