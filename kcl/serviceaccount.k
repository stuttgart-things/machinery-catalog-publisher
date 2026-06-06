"""
ServiceAccount for machinery-catalog-publisher.

The publisher reads machinery's gRPC API and writes to S3 — it never touches
the cluster API directly, so (like catalog-locator) there is no ClusterRole /
ClusterRoleBinding here.
"""

import labels

config = labels.config

serviceAccount = {
    apiVersion: "v1"
    kind: "ServiceAccount"
    metadata: {
        name: config.name
        namespace: config.namespace
        labels: labels.commonLabels
    }
}
