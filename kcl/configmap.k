"""
ConfigMap for machinery-catalog-publisher.

Holds the non-secret publisher config rendered as config.yaml, mounted at
/etc/publisher/config.yaml (see deploy.k). Secret material (S3 endpoint,
credentials, CA bundle) is NOT here — it comes from the connection Secret
via envFrom. This mirrors catalog-locator, which likewise keeps secrets out
of the ConfigMap.
"""

import labels
import yaml

config = labels.config

# The publisher config document, encoded to YAML for the mounted file.
_publisherConfig = {
    interval = config.interval
    owner = config.owner
    source = {
        machineryAddr = config.machineryAddr
        kinds = config.kinds
    }
    sink = {
        bucket = config.bucket
        keyPrefix = config.keyPrefix
        layout = config.layout
        entityNamespace = config.entityNamespace
    }
}

configMap = {
    apiVersion: "v1"
    kind: "ConfigMap"
    metadata: {
        name: "{}-config".format(config.name)
        namespace: config.namespace
        labels: labels.commonLabels
    }
    data: {
        "config.yaml": yaml.encode(_publisherConfig)
    }
}
