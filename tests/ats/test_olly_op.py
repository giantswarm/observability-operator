import logging
from typing import List

import os
import pykube
import pytest

from pytest_helm_charts.clusters import Cluster
from pytest_helm_charts.giantswarm_app_platform.app import (
  AppFactoryFunc,
  ConfiguredApp
)
from pytest_helm_charts.k8s.deployment import wait_for_deployments_to_run
from pytest_helm_charts.k8s.namespace import ensure_namespace_exists

logger = logging.getLogger(__name__)

timeout: int = 300

monitoring_namespace = "monitoring"
giantswarm_namespace = "giantswarm"
deployment_name = "observability-operator"

@pytest.fixture(scope="module")
def certmanager(app_factory: AppFactoryFunc) -> ConfiguredApp:
    """
    Deploy cert-manager.
    """
    app_factory(
        "cert-manager-app",
        "3.9.0",
        catalog_name="giantswarm-catalog",
        catalog_namespace=giantswarm_namespace,
        catalog_url="https://giantswarm.github.io/giantswarm-catalog/",
        namespace=giantswarm_namespace,
        deployment_namespace="kube-system",
    )

@pytest.fixture(scope="module")
def observabilityOperator(kube_cluster: Cluster, app_factory: AppFactoryFunc, certmanager: ConfiguredApp) -> ConfiguredApp:
    """
    Deploy observability-operator.
    """
    ensure_namespace_exists(kube_cluster.kube_client, monitoring_namespace)
    app_factory(
        deployment_name,
        os.environ["ATS_CHART_VERSION"],
        catalog_name="control-plane-test-catalog",
        catalog_namespace=giantswarm_namespace,
        catalog_url="https://giantswarm.github.io/control-plane-test-catalog",
        namespace=giantswarm_namespace,
        deployment_namespace=giantswarm_namespace,
    )

@pytest.fixture(scope="module")
def deployments(kube_cluster: Cluster, observabilityOperator: ConfiguredApp) -> List[pykube.Deployment]:
    logger.info("create mandatory grafana secrets for deployment")
    grafanaSecretObject = {
    "apiVersion": "v1",
    "kind": "Secret",
    "metadata": {
        "name": "grafana",
        "namespace": monitoring_namespace
    },
    "type": "Opaque",
    "data": {
        "admin-password": "YWRtaW4=",
        "admin-user": "YWRtaW4=",
      },
    }

    grafanaSecret = pykube.Secret(kube_cluster.kube_client, grafanaSecretObject)
    if not grafanaSecret.exists():
        grafanaSecret.create()

    # This is a dummy openssl generated x509 certificate using `openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -sha256 -days 365 -nodes`
    grafanaTLSSecretObject = {
    "apiVersion": "v1",
    "kind": "Secret",
    "metadata": {
        "name": "grafana-tls",
        "namespace": monitoring_namespace
    },
    "type": "Opaque",
    "data": {
        "tls.crt": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUZaVENDQTAyZ0F3SUJBZ0lVSERBNyt2ejBVTzhaNUNwSG1VdmVtcFZrbUIwd0RRWUpLb1pJaHZjTkFRRUwKQlFBd1FqRUxNQWtHQTFVRUJoTUNXRmd4RlRBVEJnTlZCQWNNREVSbFptRjFiSFFnUTJsMGVURWNNQm9HQTFVRQpDZ3dUUkdWbVlYVnNkQ0JEYjIxd1lXNTVJRXgwWkRBZUZ3MHlOREV4TURReE1USTBNVGxhRncweU5URXhNRFF4Ck1USTBNVGxhTUVJeEN6QUpCZ05WQkFZVEFsaFlNUlV3RXdZRFZRUUhEQXhFWldaaGRXeDBJRU5wZEhreEhEQWEKQmdOVkJBb01FMFJsWm1GMWJIUWdRMjl0Y0dGdWVTQk1kR1F3Z2dJaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQwpEd0F3Z2dJS0FvSUNBUUNXNFVFaVhBd0ZpaVJIcE0ydFVzbjFTSnZId1RHcmluUnFhV3VGeXc0aGZjUzdJQWNvCnRENGQ3bzFuYllvSWwvMklTQWxUWllzYWtDNW0zWGhucjVuY3NXSStOenAvMVc4YkhIU1V5VVd1Wnpzb2dKVngKcXU2SWhzL05ZU2FDU2xyOTd5Rkd4YXBya0t4NWgwZ2ZEN2lIOFpENng5Y3RlRnlrTHVEdFViTG1CcWNkVEtkZwp2RU1TUjBtNmxjb2M5ZkRHUVkrUXJpdVJLSzZiQUYzcXVOM04zR1IzSzlKWUtudjA0cEM4eUZJK2NLTFZaRU9CClVzaTd6OVpJbGhFZHl6QW9NWERDeXBNZWpxVDBSWmJWeUtGeXcvYWEzYVhmV2RSdmF3NU1SUE41cTdaZmNsQUYKMWN6UjVON0I4UWFNUnlZVzJKakpwYUh1RkFBTitxMlRBdlZSRENNdXJYQVFsNEsyOWpQTkdxdU4vM0NLczNOegpoaldleVRUYWZ1THBGemZJTGZaVndOdEVmWldFc21lUTJWbVhzcU1SOG14ZXR0Z0tsaE1WaGpKb25LSy9YYzhJCkkreXRzSHlDUHdlbGJId05VVXJTSjdpQ2NZcktJRFZFZ1ZGQzJGQXFaWTJ3bithSlk4c0NFNlhoY1ZlYXNYankKSGpHTEs2K0ZqcUFOUVpkTFBWbzkzZVZoQThlTWJ2UlFiR0dqS2ZYMWN3Z1dOWnhkT0JYUngwYjZjRm1STUMyWQp4bW9SaFhScTlVL3RhNXk0Z3Y2MmRDdGVFYkd2d1Z2cVhVVEt3QTRBWnJWT3RibysxMnMwWjFZNDVpUGkyT0pLCkhOSEVadWFGVDE0b29kclZ3VllwUDNHbjJiYnZpSlZwT2M2V0JMWkVKNkpCOEZhTGpnMCs3VENUUVFJREFRQUIKbzFNd1VUQWRCZ05WSFE0RUZnUVVQWng1SFdQNVErTlpjeGFDNU40NG1COGROU0V3SHdZRFZSMGpCQmd3Rm9BVQpQWng1SFdQNVErTlpjeGFDNU40NG1COGROU0V3RHdZRFZSMFRBUUgvQkFVd0F3RUIvekFOQmdrcWhraUc5dzBCCkFRc0ZBQU9DQWdFQVhKRmcvRG44VzBrQjhxYzFpTHgzSDRxOENrWkFQNk1GRUVYU3R1dDBINmFxSFVFR2IrVUYKTmUweXZ6N3JRSThYdmpsYS94SmQwSm5YUXg4OXVQOERQM2I5WnFJSmR3VVlqREhOQjZoRUZRVUUxb1ZxbEc2WgpBdERnWUJCd2p2Q0dMckFxZ3hxaTIxOUxzbTJiMU1ZS1J0QUFQUkhiN1krL01pRTBQU2hMc2hBaTBRSXVvU1RNCmRldlRkcHpmTkRJclRMNGNQbTJ0TDJHVXFWbVo5b3kzTUJ2VTJuT0xLZHFFaThmN29rejRyNm50S3NjZGJsZEsKZEJ4V2VFTmxnQUYwUjQ0dmxVanExMVV2RnRiQ09sMnlod2drY29UNmx1RlpLN2ZVblVYaEFzdWp1WlRVVlBBWgpFSGprRXI2Q0gyNm5VbnphcnByOUlSZ2dYTGpyR3U5NjE0dk9SWmRUd0J4MVpnZjF2UzlpdllHYXlQQmdHdlMzCnBYeFZjNFpvR2dDUUZSZlZVRXViZHNjUTcxUHAvK0xHNEl1WTFZTGNUZWFIKzZkcmFZOWJxSWhWY21wRS9ldUkKeTh6MzRoSlY2M3o4aXBYeFlnY09qblJBcmRtRE40ZlJXY2haZFZiZGV2UENMSjFLTmpmZ01mbkNVWEkya3FBcApPUDVaRURxR3NyQVFKNHRzZGpsVkF0UXVTS3NCZkhaeWFLcG9mVzArWm9GNk02SjJWNnVyMUk5T1kyR1NRNGw3CmZkb3Z3Y0VRMGlETXVPRWFaaGxRbmVrdGhrM2VkREdlVDJNSWlvakl1Y3VVbVJxWU56ZThmREVTVFRRQTBTamQKWGd6bFJwZ0ZQb2lLNFFvbTJBRzNSeThtS2MwQTRzTHlUMmwxbW1SZFFYcmpXZHRUZFh6WGpoND0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=",
        "tls.key": "LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JSUpRZ0lCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQ1N3d2dna29BZ0VBQW9JQ0FRQ1c0VUVpWEF3RmlpUkgKcE0ydFVzbjFTSnZId1RHcmluUnFhV3VGeXc0aGZjUzdJQWNvdEQ0ZDdvMW5iWW9JbC8ySVNBbFRaWXNha0M1bQozWGhucjVuY3NXSStOenAvMVc4YkhIU1V5VVd1Wnpzb2dKVnhxdTZJaHMvTllTYUNTbHI5N3lGR3hhcHJrS3g1CmgwZ2ZEN2lIOFpENng5Y3RlRnlrTHVEdFViTG1CcWNkVEtkZ3ZFTVNSMG02bGNvYzlmREdRWStRcml1UktLNmIKQUYzcXVOM04zR1IzSzlKWUtudjA0cEM4eUZJK2NLTFZaRU9CVXNpN3o5WklsaEVkeXpBb01YREN5cE1lanFUMApSWmJWeUtGeXcvYWEzYVhmV2RSdmF3NU1SUE41cTdaZmNsQUYxY3pSNU43QjhRYU1SeVlXMkpqSnBhSHVGQUFOCitxMlRBdlZSRENNdXJYQVFsNEsyOWpQTkdxdU4vM0NLczNOemhqV2V5VFRhZnVMcEZ6ZklMZlpWd050RWZaV0UKc21lUTJWbVhzcU1SOG14ZXR0Z0tsaE1WaGpKb25LSy9YYzhJSSt5dHNIeUNQd2VsYkh3TlVVclNKN2lDY1lySwpJRFZFZ1ZGQzJGQXFaWTJ3bithSlk4c0NFNlhoY1ZlYXNYanlIakdMSzYrRmpxQU5RWmRMUFZvOTNlVmhBOGVNCmJ2UlFiR0dqS2ZYMWN3Z1dOWnhkT0JYUngwYjZjRm1STUMyWXhtb1JoWFJxOVUvdGE1eTRndjYyZEN0ZUViR3YKd1Z2cVhVVEt3QTRBWnJWT3RibysxMnMwWjFZNDVpUGkyT0pLSE5IRVp1YUZUMTRvb2RyVndWWXBQM0duMmJidgppSlZwT2M2V0JMWkVKNkpCOEZhTGpnMCs3VENUUVFJREFRQUJBb0lDQUFPYzFTQnJmSmxtOVBhZ2x5cUxwdXl3CkwvcXlkRkpvWHEwNHkxNnZpbUVUdGo2V1g3dm9rYmo4ODBOcGhHM0pERVNnUzFXcmRmV2FFSldRV01leXlFZngKc3BBYUFBZ3V2N04rNGJvZjRzK2piU0xMaDhpRHc2VEZPTWlLaDF4UnJkdEY0Vkd4c1NLSHprWTEyMnlmeGIxWQpXaTVUMDI2KzRxS1R3Wkw1NndNUytiU1ZmYTRWUnhxbUl4L01MeXBPV3RVZUEvTXhzZ1BCdzV5RFRJa3VvekR6CjN5OG1mM1p5a1NyWkdlMUxRRFo2aGxoazRQMjA2N3ZwNU56TUVYYy85ZFJ0VjIxeWxSSnAxWTFJanVCcDgyT3QKU1lSUzlrNzc5bkhNZzU2NlNZVlhGVFFCZEV0MTMzZWI2dVMzZ2VVSGtMQzF2OExLckJXQ0QzeXNtZWRhMU1jLwpBSE5VZ2NicE5vdUJIblI5ditzUmlDaStMMklERDZPTlU5enJvTSt4eTlTWTRXbEFYSjNyYWNFYnBmOXdwcVc1CkRtaEoxWW5ldGFBamUwb1c4cHQ5VWlVbHpjeFl5bW5NK0tNM3V3NW43RGU4dGpLeDlBRlc1QlN4VjdqOXZSOGQKcmdjdVZpcWNLNUdoL3pBZGpxUStXajlSR0o1MjRxcmc4UEh0Vnl0RDJVazdwLy9Na2Y1SjRxc1lrTUZpd2dFNQpTaDhsRnZVZ0I4Q0ZjdmYwV1djVGFRcWZiMXNwRm5tNDBwendxQXQ0VjF0UW9DZ3hzZmdPcVJTMmo4TXFtdVU5ClIwVTZ6NDVFU0l0MkpsZ3plbXFvQ0oyMjZacTJMc3lBOEdGVjhNOHRiSkg1cHU5TW40QTR0WngwWHpscHZNTngKbTRBSCs0U1htT1FlUGhNa0c5STVBb0lCQVFES0VWaFVsNXc3aUZodWJMYVZDWkVSaFdHUThZMWhzSXhjSnpBcgpBTWJhYTlnczQ1bWNNT3hoSFZENDRiaVJQQUFjZGVYWEIxeEhtVWZXdklXaDRBVjVrbEYxZ3NFRkwvWkM3amNPClVtQ1RXSFk3WkpVdkk1UnJKeDBCa2JHbjFSQ0Mrajl4SW1IdFlkUlZ5bm1JWUY0U2k5czY1cUFxbzBON1BDVDIKSXZpZlJ0cy8rTEZUM2J5WEpjWE1Ua2RnclRxcFcvUnZLblgwbUIwZ1V5cVlZcC9xT2w3citTeHZqaUg1TFEvRwpXQmZ3c1lXanFJOG5jVDZHK0x3TzkxNkE0TnplcC9SczZLRVU2WmswcmVCbmU5bFpvNmgzRW93WkhQTlVURWdNCi9oOVVYNHhpTlZIRTVFMlk5TkYzb2s4Y0l3R3lxZVphcHFGUkZXY2RZdXBsVmdVM0FvSUJBUUMvSm1kemZEWkYKQ0FWeC9SNVJ3S2ZMWk1EZFVMcWNENU41dmtYN1BuT0RTUm1XUDFsQWZHeFY4cnZBTWI5eG1yRkgyUWlSZUtKMwpyaVVyUEJLZ2FDMC9QOHRNSGJtK2pscDVHWlczenhhTlo4WHJHaFZGUmlhRmFEOHhHcHlZYUFZR01HSzBvOHViCnBrZnpzMUVtaGVObFRvS09SU0RrOHhMMStpOTU5elRtZTNYbnBUNUU5Um1VTjRKZk1xdWM3eE9Xc3g4NkU3QTIKQ24xL3FvcFNHczVKU0xHaVRzRTYvU1pJS1hFNmpiVHJhb0VlRVpEb2lENGNEdDJ5MWxVTkQ3VjRnNWNXdGR2SwpZWjlqYUU2dFppRkpSblBCQURsQXZFWkFFb2JoazVvaUlKeitacE5zWkR2ZjRWRFdtUVN2ZlM5d2J5aHd1QThNCnBKTHQvYktacEdkSEFvSUJBUUNaVktRVVBKOCt4VzFsRFhWV2psWFlWNy83UG5Bc0NzM1hONTFqWkVtQWdJa2YKTnUzZkNYaTFFSXZhNzEwZ1I4bEZ6MmpDekVFSHk1WXBxaEExRDByWVAyRTByQzFQaEY5MzFrOU12Tkd2dlZhcQpKdDdWVUVoVkx1N2h1KzUxRGtaalBRVmJFZDRCUlZUY2JMSGYvRkFsL3A0eWljSkwyR3RpWGZjbUZzOVYvV3h0CmxJYnF2cjFXYkVFMWtNaTA0WDQ3K1J6NEpkNHY1aVZqMi9mY2Zpb2VrSUJxeXo4ZXYxbWtQTDlWb0k4NkExc0gKSHViUjVTcXZQSnRuTitvc0hYVThOM0pRR2czeFVua0E4bGZ2N3BpMVhteDlQa08wNHJUNTZKQmIyNUZtY3NLUgoyeWZiSWVxSUFHM2FPLzJQdWppVm5EckIydU5hSmNXREZRWU1NMFB0QW9JQkFCM3BGWUVGcTd4TGtBYVJOQUJZCjVDaTRnRkZoUTRJT0VlYVg5bjFrL1ZCS3pQMHN0bnVYdktBS1ZvU3hoQ0p6c1Uvcnp5SnNQUWY3TVFlOTQ3QkQKL09pTHcvVUtKYm1DcnZlS0lGcGR4a3FrTlYwZmZMcVZTb3ZQanl2UTNUWWYrT2xaQXVqL1JHbjdzQUNiUzVSNgptT1dPVG5HU3NaNEJ4ekxFVGsrSWRqZW1sQUdHWXVNSmMxSTFDV3A2RkU1L1BwSnpQdXlvamdjMnh3S0dIaFRRCjZ2eWVxbVVhYTdRZVRySTJBZWpHcWN1NG83R2YwQXdDM2Ewb1NscWtuVFJwQTkxeXdkNms0RnFmd2dBZEgwcVUKMDVxU3NxUTlzN2ZFZmoyaWFJYTl3UDJjR3RUWUdqTjR6OEd0a1NlelUvOWQyR0dBazFSb0NMclN1Y2NSenJPcAovUnNDZ2dFQUxacDRuVmFNN3RqT25WdUV6NWFSQm9TcisrTDIvVnFLNTgwOU84ZStNK0ZqVkVKaGZBVU8rMWN1ClRBSkZLUmNtWkJpRStIN1hxMlZyMFpzdzArYUJ0TWdXbGNFRlRIZFVKQVU4WUtPWmNYL0NGTEtRYU5KKzVEVlkKNGhmS1hYRExYTnZYU0k5REdtZ2pTc3ROc1dXaXZNT3IwYTBjWW16TmZPZEkxRlNXNFlia3NQNWtheDJkVkFPcgpESzZFS0dnZ2hFekNwKzAzNk8rUUNoNlBBUnZxUVZLK1hScHZTbFpRNUxObUxuR2JpeHpuNlVpYmRmaEQxWkZoCm1RTTlabWp4UXgrK0NnM1BPRWJyQVRNYXdNNnRTVVF0UnZKb21hQmpRbFcyNjdGSUhyQ2EwRTRpdnVXNnVVancKeHJjK1hEVy9lam95akI4cXJvK1U4YnpzYy9ZU1hRPT0KLS0tLS1FTkQgUFJJVkFURSBLRVktLS0tLQo=",
      },
    }

    grafanaTLSSecret = pykube.Secret(kube_cluster.kube_client, grafanaTLSSecretObject)
    if not grafanaTLSSecret.exists():
        grafanaTLSSecret.create()

    logger.info("waiting for observability-operator deployment")
    deployment_ready = wait_for_deployments_to_run(
        kube_cluster.kube_client,
        [deployment_name],
        monitoring_namespace,
        timeout,
    )
    logger.info("observability-operator deployment is ready")

    return deployment_ready

@pytest.fixture(scope="module")
def pods(kube_cluster: Cluster, deployments: List[pykube.Deployment]) -> List[pykube.Pod]:
    pods = pykube.Pod.objects(kube_cluster.kube_client)

    pods = pods.filter(
        namespace=monitoring_namespace,
        selector={
            "app.kubernetes.io/name": deployment_name,
            "app.kubernetes.io/instance": deployment_name,
        },
    )

    return pods

@pytest.mark.smoke
@pytest.mark.upgrade
def test_api_working(kube_cluster: Cluster) -> None:
    """
    Testing apiserver availability.
    """
    assert kube_cluster.kube_client is not None
    assert len(pykube.Node.objects(kube_cluster.kube_client)) >= 1

@pytest.mark.smoke
@pytest.mark.upgrade
def test_pods_available(deployments: List[pykube.Deployment], pods: List[pykube.Pod]) -> None:
    for s in deployments:
        assert int(s.obj["status"]["readyReplicas"]) == int(s.obj["spec"]["replicas"])
