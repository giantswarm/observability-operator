#!/bin/bash

set -eu

# Basic auth credentials
kubectl create secret generic mimir-gateway-basic-auth-secret --dry-run=client -oyaml --from-file .htpasswd > mimir-gateway-basic-auth.secret.yaml

# Authorized tenants
kubectl create secret generic mimir-gateway-authorized-tenants --dry-run=client -oyaml --from-file authorized_tenants.map > mimir-gateway-authorized-tenants.secret.yaml

# Reload scripts
kubectl create configmap mimir-gateway-reload --dry-run=client -oyaml --from-file=config-watcher.sh --from-file=nginx-reload.sh > mimir-gateway-reload.configmap.yaml

kubectl apply \
  -f mimir-gateway-basic-auth.secret.yaml \
  -f mimir-gateway-authorized-tenants.secret.yaml \
  -f mimir-gateway-reload.configmap.yaml
