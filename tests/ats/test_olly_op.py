import logging
from typing import List

import pykube
import pytest

from pytest_helm_charts.clusters import Cluster
from pytest_helm_charts.k8s.deployment import wait_for_deployments_to_run
from pytest_helm_charts.giantswarm_app_platform.app import (
    wait_for_apps_to_run,
    AppFactoryFunc,
    AppCR,
)
from pytest_helm_charts.utils import YamlDict

logger = logging.getLogger(__name__)

namespace_name = "monitoring"
deployment_name = "observability-operator"
timeout: int = 560


@pytest.mark.smoke
def test_api_working(kube_cluster: Cluster) -> None:
    """
    Testing apiserver availability.
    """
    assert kube_cluster.kube_client is not None
    assert len(pykube.Node.objects(kube_cluster.kube_client)) >= 1

@pytest.fixture(scope="module")
def deployments(kube_cluster: Cluster) -> List[pykube.Deployment]:
    logger.info("create mandatory secrets for deployment")
    grafanaSecret = {
    "apiVersion": "v1",
    "kind": "Secret",
    "metadata": {
        "name": "grafana",
        "namespace": "monitoring"
    },
    "type": "Opaque":
    "data": {
        "admin-password": "YWRtaW4=",
        "admin-user": "YWRtaW4="
      },
    }

    pykube.Secret(kube_cluster.kube_client, grafanaSecret).create()

    grafanaTLSSecret = {
    "apiVersion": "v1",
    "kind": "Secret",
    "metadata": {
        "name": "grafana-tls",
        "namespace": "monitoring"
    },
    "type": "Opaque":
    "data": {
        "tls.crt": "YWRtaW4=",
        "tls.key": "YWRtaW4="
      },
    }
    pykube.Secret(kube_cluster.kube_client, grafanaTLSSecret).create()

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
def pods(kube_cluster: Cluster, deployments: List[pykube.Deployment]) -> List[pykube.Pod]:
    pods = pykube.Pod.objects(kube_cluster.kube_client)

    pods = pods.filter(
        namespace=namespace_name,
        selector={
            "app.kubernetes.io/name": "observability-operator",
            "app.kubernetes.io/instance": "observability-operator",
        },
    )

    return pods


# when we start the tests on circleci, we have to wait for pods to be available, hence
# this additional delay and retries
@pytest.mark.smoke
@pytest.mark.upgrade
@pytest.mark.flaky(reruns=5, reruns_delay=10)
def test_pods_available(deployment: List[pykube.Deployment]):
    for s in deployment:
        assert int(s.obj["status"]["readyReplicas"]) == int(s.obj["spec"]["replicas"])
