"""
Deployment for machinery-catalog-publisher.

Mounts the rendered config.yaml from the ConfigMap and pulls the S3/MinIO
connection secret in via envFrom. replicas: 0 acts as "suspend". The Stakater
Reloader annotation (optional) restarts the Pod when the ConfigMap or the
connection Secret changes, so edits take effect without a manual rollout.
"""

import labels

config = labels.config

# Reloader watches the ConfigMap + connection Secret referenced below.
_reloaderAnnotations = {
    "configmap.reloader.stakater.com/reload": "{}-config".format(config.name)
    "secret.reloader.stakater.com/reload": config.connectionSecret
} if config.reloaderEnabled else {}

deployment = {
    apiVersion: "apps/v1"
    kind: "Deployment"
    metadata: {
        name: config.name
        namespace: config.namespace
        labels: labels.commonLabels
        annotations: _reloaderAnnotations
    }
    spec: {
        replicas: config.replicas
        selector: {
            matchLabels: labels.selectorLabels
        }
        strategy: {
            # The publisher holds a single watch stream + in-memory state;
            # avoid two writers racing the same keys during a rollout.
            type: "Recreate"
        }
        template: {
            metadata: {
                labels: labels.commonLabels
            }
            spec: {
                serviceAccountName: config.name
                containers: [
                    {
                        name: config.name
                        image: config.image
                        imagePullPolicy: config.imagePullPolicy
                        ports: [
                            {
                                name: "metrics"
                                containerPort: config.metricsPort
                                protocol: "TCP"
                            }
                        ]
                        env: [
                            {
                                name: "CONFIG_FILE"
                                value: "/etc/publisher/config.yaml"
                            }
                            {
                                name: "METRICS_ADDR"
                                value: ":{}".format(config.metricsPort)
                            }
                        ] + [
                            {name: k, value: v}
                            for k, v in config.extraEnvVars
                        ]
                        # S3/MinIO connection secret → S3_* env vars.
                        envFrom: [
                            {
                                secretRef: {
                                    name: config.connectionSecret
                                }
                            }
                        ]
                        resources: {
                            requests: {
                                cpu: config.cpuRequest
                                memory: config.memoryRequest
                            }
                            limits: {
                                cpu: config.cpuLimit
                                memory: config.memoryLimit
                            }
                        }
                        livenessProbe: {
                            httpGet: {
                                path: "/healthz"
                                port: "metrics"
                            }
                            initialDelaySeconds: 10
                            periodSeconds: 15
                            timeoutSeconds: 5
                            failureThreshold: 3
                        }
                        readinessProbe: {
                            httpGet: {
                                path: "/healthz"
                                port: "metrics"
                            }
                            initialDelaySeconds: 5
                            periodSeconds: 10
                            timeoutSeconds: 3
                            failureThreshold: 3
                        }
                        securityContext: {
                            readOnlyRootFilesystem: True
                            runAsNonRoot: True
                            allowPrivilegeEscalation: False
                            capabilities: {
                                drop: ["ALL"]
                            }
                        }
                        volumeMounts: [
                            {
                                name: "config"
                                mountPath: "/etc/publisher"
                                readOnly: True
                            }
                            {
                                name: "tmp"
                                mountPath: "/tmp"
                            }
                        ]
                    }
                ]
                securityContext: {
                    runAsNonRoot: True
                    runAsUser: 65532
                    runAsGroup: 65532
                    fsGroup: 65532
                    seccompProfile: {
                        type: "RuntimeDefault"
                    }
                }
                volumes: [
                    {
                        name: "config"
                        configMap: {
                            name: "{}-config".format(config.name)
                        }
                    }
                    {
                        name: "tmp"
                        emptyDir: {}
                    }
                ]
            }
        }
    }
}
