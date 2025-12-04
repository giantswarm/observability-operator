#!/bin/bash
set -e

# Patch CRDs with configurations that controller-gen doesn't generate
CRD_FILE="config/crd/bases/observability.giantswarm.io_grafanaorganizations.yaml"

echo "Patching CRDs with manual configurations..."

# Function to add conversion webhook configuration
add_conversion_webhook() {
    echo "Adding conversion webhook configuration to $CRD_FILE"
    
    # Check if conversion config already exists
    if grep -q "conversion:" "$CRD_FILE"; then
        echo "Conversion webhook configuration already exists"
        return 0
    fi
    
    # Add conversion webhook config after "scope: Cluster"
    sed -i '/scope: Cluster/a\
  conversion:\
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
}

# Function to add MCB deployment comment
add_mcb_comment() {
    echo "Adding MCB deployment comment to $CRD_FILE"
    
    # Check if comment already exists
    if head -n 5 "$CRD_FILE" | grep -q "management-cluster-bases"; then
        echo "MCB deployment comment already exists"
        return 0
    fi
    
    # Add comment at the beginning of the file
    sed -i '1i\
# This secret is deployed via https://github.com/giantswarm/management-cluster-bases/blob/16e623dd03558a616fe92641dfbdd79b8807d462/bases/crds/giantswarm/kustomization.yaml#L11\
# If you edit this CRD, do not forget to edit the link to this CRD in MCB' "$CRD_FILE"
    
    echo "MCB deployment comment added successfully"
}

# Apply all patches
add_conversion_webhook
add_mcb_comment

echo "CRD patching completed successfully"
