"""
Schema definition for machinery-catalog-publisher deployment configuration.

Unlike machinery / catalog-locator there is no inbound gRPC or HTMX server:
the publisher is a one-way sync (machinery gRPC → S3) and only exposes an
HTTP metrics/health port. The publisher's own config (interval, source,
sink) is rendered into a config.yaml ConfigMap (see configmap.k); the S3
connection secret is wired in via envFrom (see deploy.k).
"""

schema CatalogPublisher:
    """Configuration for machinery-catalog-publisher Kubernetes deployment"""

    # Metadata
    name: str = "machinery-catalog-publisher"
    namespace: str = "machinery-catalog-publisher"

    # Image
    image: str = "ghcr.io/stuttgart-things/machinery-catalog-publisher:latest"
    imagePullPolicy: str = "IfNotPresent"

    # Replicas (0 == suspend) and resources
    replicas: int = 1
    cpuRequest: str = "50m"
    cpuLimit: str = "300m"
    memoryRequest: str = "64Mi"
    memoryLimit: str = "256Mi"

    # Metrics / health HTTP port (Prometheus + /healthz)
    metricsPort: int = 8080

    # ---- publisher config (rendered into config.yaml) ----
    interval: str = "5m"
    owner: str = "platform-team"
    machineryAddr: str = "machinery.machinery-system.svc:50051"
    kinds: [str] = ["*"]
    bucket: str = ""
    keyPrefix: str = "status/"
    layout: str = "PerResource"        # PerResource | Aggregate
    entityNamespace: str = "default"

    # ---- S3/MinIO connection secret ----
    # Secret holding S3_ENDPOINT / S3_ACCESS_KEY / S3_SECRET_KEY / S3_CA_BUNDLE
    # / S3_INSECURE_SKIP_VERIFY, consumed via envFrom. Materialized externally
    # (ESO / Kyverno / kubectl); this schema only references it by name.
    connectionSecret: str = ""

    # Service
    serviceType: str = "ClusterIP"
    serviceEnabled: bool = True        # expose metrics for scraping

    # Stakater Reloader: restart the Pod when the ConfigMap or connection
    # Secret changes, so config edits take effect without a manual rollout.
    reloaderEnabled: bool = True

    # Extra environment variables (e.g. METRICS_ADDR override)
    extraEnvVars: {str:str} = {}

    # Labels and annotations
    labels: {str:str} = {}
    annotations: {str:str} = {}

    check:
        replicas >= 0, "replicas must be >= 0"
        metricsPort >= 1 and metricsPort <= 65535, "metricsPort must be valid"
        name != "", "name must not be empty"
        namespace != "", "namespace must not be empty"
        bucket != "", "bucket must not be empty"
        connectionSecret != "", "connectionSecret (S3/MinIO) must be set"
        layout in ["PerResource", "Aggregate"], "layout must be PerResource or Aggregate"
