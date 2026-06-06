"""
Common labels and command-line option wiring for machinery-catalog-publisher.
"""

import schema

# Command-line options (-D config.x=...)
_image = option("config.image")
_namespace = option("config.namespace")
_replicas = option("config.replicas")
_metricsPort = option("config.metricsPort")
_interval = option("config.interval")
_owner = option("config.owner")
_machineryAddr = option("config.machineryAddr")
_kinds = option("config.kinds")
_bucket = option("config.bucket")
_keyPrefix = option("config.keyPrefix")
_layout = option("config.layout")
_entityNamespace = option("config.entityNamespace")
_connectionSecret = option("config.connectionSecret")
_serviceType = option("config.serviceType")
_serviceEnabled = option("config.serviceEnabled")
_reloaderEnabled = option("config.reloaderEnabled")
_extraEnvVars = option("config.extraEnvVars") or {}
_labels = option("config.labels") or {}

# Config instance with command-line overrides
config: schema.CatalogPublisher = schema.CatalogPublisher {
    if _image:
        image = _image
    if _namespace:
        namespace = _namespace
    if _replicas != None and _replicas != Undefined:
        replicas = int(_replicas)
    if _metricsPort:
        metricsPort = int(_metricsPort)
    if _interval:
        interval = _interval
    if _owner:
        owner = _owner
    if _machineryAddr:
        machineryAddr = _machineryAddr
    if _kinds:
        kinds = _kinds
    if _bucket:
        bucket = _bucket
    if _keyPrefix:
        keyPrefix = _keyPrefix
    if _layout:
        layout = _layout
    if _entityNamespace:
        entityNamespace = _entityNamespace
    if _connectionSecret:
        connectionSecret = _connectionSecret
    if _serviceType:
        serviceType = _serviceType
    if _serviceEnabled != None and _serviceEnabled != Undefined:
        serviceEnabled = _serviceEnabled in [True, "True", "true"]
    if _reloaderEnabled != None and _reloaderEnabled != Undefined:
        reloaderEnabled = _reloaderEnabled in [True, "True", "true"]
    if _extraEnvVars:
        extraEnvVars = _extraEnvVars
    if _labels:
        labels = _labels
}

# Common labels applied to all resources
commonLabels = {
    "app.kubernetes.io/name": config.name
    "app.kubernetes.io/instance": config.name
    "app.kubernetes.io/component": "status-publisher"
    "app.kubernetes.io/part-of": "stuttgart-things"
    **config.labels
}

# Selector labels for pods
selectorLabels = {
    "app.kubernetes.io/name": config.name
    "app.kubernetes.io/instance": config.name
}
