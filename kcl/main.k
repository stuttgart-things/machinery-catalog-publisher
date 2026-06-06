"""
Main entry point for machinery-catalog-publisher Kubernetes deployment.
Imports all resources and exports manifests (filtering out None/Undefined).
"""

import namespace
import serviceaccount
import configmap
import deploy
import service

manifests = [
    m for m in [
        namespace.namespace
        serviceaccount.serviceAccount
        configmap.configMap
        deploy.deployment
        service.service
    ] if m
]
