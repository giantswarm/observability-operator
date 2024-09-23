#!/bin/bash

# Helper function - prints an error message and exits
exit_error() {
  echo "Error: $*"
  exit 1
}

# Helper function - clean up the WC
clean_wc() {
  kubectl delete -f grizzly-e2e-wc.yaml
  rm grizzly-e2e-wc.yaml
}

# Helper function - checks the existence of the cm and secret for either alloy or prometheus-agent
check_configs() {
  echo "Checking if the corresponding $1-$2 has been created"

  config=$(kubectl get $2 -n org-giantswarm ollyoptest-$1-$2)

  [[ -z "$config" ]] && echo "$1-$2 not found" || echo "$1-$2 found. Test succeeded"
}

main() {
  [[ -z "$1" ]] && exit_error "Please provide the installation name as an argument"

  # Logging into the specified installation to perform the tests
  tsh kube login $1

  echo "Checking if observability-operator app is in deployed state"

  deployed=$(kubectl get app -n giantswarm observability-operator -o yaml | yq .status.release.status)

  [[ "$deployed" != "deployed" ]] \
    && exit_error "observability-operator app is not in deployed state. Please fix the app before retrying" || echo "observability-operator app is indeed in deployed state"

  echo "Creating WC"

  # Getting latest Giant Swarm release version
  toUseRelease="$(kubectl gs get releases -o template='{{range .items}}{{.status.ready}}/{{.metadata.name}}
  {{end}}' | sed -ne 's/false\/aws-//p' | sort -V | tail -1)"

  # Creating WC template and applying it
  kubectl gs template cluster --provider capa --name ollyoptest --organization giantswarm --description "observability-operator e2e tests" --release $toUseRelease > grizzly-e2e-wc.yaml
  kubectl create -f grizzly-e2e-wc.yaml

  echo "WC named 'ollyoptest' created. Waiting for it to be ready"

  sleep 1200

  echo "Checking if the metrics agent is up and running on the WC"

  # Logging into the WC to get the context into the kubeconfig
  tsh kube login $1-ollyoptest
  tsh kube login $1

  agent=$(kubectl get apps -n org-giantswarm | grep ollyoptest-prometheus-agent)
  alloy=$(kubectl get apps -n org-giantswarm | grep ollyoptest-alloy)

  if [[ ! -z "$agent" ]]; then
    local podStatus=$(kubectl get pods -n kube-system --context teleport.giantswarm.io-$1-ollyoptest prometheus-prometheus-agent-0 -o yaml | yq .status.phase)
    
    [[ "$podStatus" != "Running" ]] && echo "prometheus-agent app deployed but pod isn't in a running state" || echo "prometheus-agent app is deployed and pod is running"
    
    check_configs "remote-write" "cm"
    check_configs "remote-write" "secret"
  elif [[ ! -z "$alloy" ]]; then
    local podStatus=$(kubectl get pods -n kube-system --context teleport.giantswarm.io-$1-ollyoptest alloy-metrics-0 -o yaml | yq .status.phase)

    [[ "$podStatus" != "Running" ]] && echo "alloy app deployed but pod isn't in a running state" || echo "alloy app is deployed and pods are running"

    check_configs "monitoring" "cm"
    check_configs "monitoring" "secret"
  else
    echo "No metrics agent app found. Cleaning the WC"
    clean_wc
    exit 1
  fi

  echo "cleaning WC"

  clean_wc
}

main "$@"
