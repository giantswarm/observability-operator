#!/bin/bash
set -e

# Add conversion webhook configuration to GrafanaOrganization CRD
CRD_FILE="config/crd/bases/observability.giantswarm.io_grafanaorganizations.yaml"

echo "Adding conversion webhook configuration to $CRD_FILE"

# Check if conversion config already exists
if grep -q "conversion:" "$CRD_FILE"; then
    echo "Conversion webhook configuration already exists"
    exit 0
fi

# Add conversion webhook config after "scope: Cluster"
sed -i '/scope: Cluster/a\  conversion:\
    strategy: Webhook\
    webhook:\
      clientConfig:\
        service:\
          name: observability-operator-webhook-service\
          namespace: system\
          path: /convert\
      conversionReviewVersions:\
      - v1\
      - v1beta1' "$CRD_FILE"

echo "Conversion webhook configuration added successfully"