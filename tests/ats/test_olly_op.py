import logging
from typing import List
import requests

import pykube
import pytest

from pytest_helm_charts.clusters import Cluster
from pytest_helm_charts.k8s.deployment import wait_for_deployments_to_run
from pytest_helm_charts.giantswarm_app_platform.app import (
    wait_for_apps_to_run,
    AppFactoryFunc,
    AppCR,
)

logger = logging.getLogger(__name__)

namespace_name = "monitoring"
deployment_name= "observability-operator"

app_catalog_url = "oci://giantswarmpublic.azurecr.io/control-plane-catalog"
apps = [
    {
      "name": "grafana",
      "version": "2.16.3",
      "config_values": f'''grafana:
  enabled: true
  grafana:
    fullnameOverride: grafana
    ingress:
      annotations:
        cert-manager.io/cluster-issuer: letsencrypt-giantswarm
        kubernetes.io/tls-acme: "true"
      enabled: true
      hosts:
      - grafana.test.gigantic.io
      ingressClassName: nginx
      tls:
      - hosts:
        - grafana.test.gigantic.io
        secretName: grafana-tls
''',
    },
    {
      "name": "cert-manager",
      "version": "3.8.1",
      "config_values": "",
    },
    {
      "name": "ingress-nginx",
      "version": "3.9.2",
      "config_values": "",
    },
]

timeout: int = 560

@pytest.mark.smoke
def test_api_working(kube_cluster: Cluster) -> None:
    """
    Testing apiserver availability.
    """
    assert kube_cluster.kube_client is not None
    assert len(pykube.Node.objects(kube_cluster.kube_client)) >= 1

# scope "module" means this is run only once, for the first test case requesting! It might be tricky
# if you want to assert this multiple times
@pytest.fixture(scope="module")
def deployedApps(
    kube_cluster: Cluster, app_factory: AppFactoryFunc
) ->  List[AppCR]:
    for app in apps:
      try:
        app_factory(
          app["name"],
          app["version"],
          "control-plane-catalog",
          namespace_name,
          app_catalog_url,
          timeout_sec=timeout,
          namespace=namespace_name,
          deployment_namespace=namespace_name,
          config_values=app["config_values"],
        )
      except pykube.exceptions.HTTPError as e:
        if e.code == 409:
          logger.warning("App already deployed", app_name=app["name"])
        else:
          raise

    logger.info("waiting for apps to be deployed")
    deployedApp = wait_for_apps_to_run(
      kube_cluster.kube_client,
      [app["name"] for app in apps],
      namespace_name,
      timeout,
    )
    logger.info("required apps are running")
    return deployedApp

@pytest.fixture(scope="module")
def deployment(kube_cluster: Cluster, deployedApps: List[AppCR]) -> List[pykube.Deployment]:
  logger.info("waiting for observability-operator deployment")
  deployment_ready = wait_for_deployments_to_run(
    kube_cluster.kube_client,
    [deployment_name],
    namespace_name,
    timeout,
  )
  logger.info("observability-operator deployment is ready")

  return deployment_ready

@pytest.fixture(scope="module")
def pods(kube_cluster: Cluster) -> List[pykube.Pod]:
  pods = pykube.Pod.objects(kube_cluster.kube_client)

  pods = pods.filter(
    namespace=namespace_name,
    selector={
      'app.kubernetes.io/name': 'observability-operator',
      'app.kubernetes.io/instance': 'observability-operator'
    }
  )

  return pods

# when we start the tests on circleci, we have to wait for pods to be available, hence
# this additional delay and retries
@pytest.mark.smoke
@pytest.mark.upgrade
@pytest.mark.flaky(reruns=5, reruns_delay=10)
def test_pods_available(deployment: List[pykube.Deployment]):
    for s in deployment:
        assert int(s.obj["status"]["readyReplicas"]) == int(
            s.obj["spec"]["replicas"])
