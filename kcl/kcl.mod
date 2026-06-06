[package]
name = "deploy-machinery-catalog-publisher"
version = "0.1.0"
description = "KCL module for deploying machinery-catalog-publisher (status → S3) on Kubernetes"

[dependencies]
k8s = "1.31"

[profile]
entries = [
    "main.k"
]
