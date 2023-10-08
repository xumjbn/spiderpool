# Kubevirt

**简体中文** | [**English**](./kubevirt.md)

## 介绍

*Spiderpool 能保证 kubevirt vm 的 Pod 在重启、重建场景下，持续获取到相同的 IP 地址。*

## Kubevirt VM 固定地址

Kubevirt VM 固定 IP 地址与 StatefulSet 的表现形式是不一样的：

1. 对于 VM ，Pod 重启前后，其 Pod 的名字是会发生变化的，但是其对应的 VMI 不论重启与否，其名字都不会发生变化。因此，我们将会以 VM 为单位来记录其固定的 IP 地址。

2. 对于 StatefulSet，Pod 副本重启前后，其 Pod 名保持不变，我们 Spiderpool 会因此以 Pod 为单位来记录其固定的 IP 地址。

> Node: 该功能默认开启。若开启，无任何限制， VM 可通过有限 IP 地址集合的 IP 池来固化 IP 的范围，但是，无论 VM 是否使用固定的 IP 池，它的 Pod 都可以持续分到相同 IP。 若关闭，VM 对应的 Pod 将被当作无状态对待，使用 Helm 安装 Spiderpool 时，可通过`--set ipam.enableKubevirtStaticIP=false` 关闭。

## 实施要求

1. 一套 Kubernetes 集群。

2. 已安装 [Helm](https://helm.sh/docs/intro/install/)。

## 步骤

### 安装 Spiderpool

- 通过 helm 安装 Spiderpool。

```bash
helm repo add spiderpool https://spidernet-io.github.io/spiderpool
helm repo update spiderpool
helm install spiderpool spiderpool/spiderpool --namespace kube-system --set multus.multusCNI.defaultCniCRName="macvlan-ens192" 
```

> 如果您所在地区是中国大陆，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` ，以帮助您更快的拉取镜像。
>
> 通过 `multus.multusCNI.defaultCniCRName` 指定集群的 Multus clusterNetwork，clusterNetwork 是 Multus 插件的一个特定字段，用于指定 Pod 的默认网络接口。

- 检查安装完成

```bash
~# kubectl get po -n kube-system | grep spiderpool
NAME                                     READY   STATUS      RESTARTS   AGE                                
spiderpool-agent-jqx4w                   1/1     Running     0          10m
spiderpool-agent-swzvc                   1/1     Running     0          10m
spiderpool-controller-7ddb58956b-fkqss   1/1     Running     0          10m
spiderpool-init                          0/1     Completed   0          10m
spiderpool-multus-9wfg5                  1/1     Running     0          10m
spiderpool-multus-pvz5l                  1/1     Running     0          10m
```

### 安装 CNI 配置

Spiderpool 为简化书写 JSON 格式的 Multus CNI 配置，它提供了 SpiderMultusConfig CR 来自动管理 Multus NetworkAttachmentDefinition CR。如下是创建 Macvlan SpiderMultusConfig 配置的示例：

- master：在此示例用接口 `ens192` 作为 master 的参数。

```bash
MACVLAN_MASTER_INTERFACE="ens192"
MACVLAN_MULTUS_NAME="macvlan-$MACVLAN_MASTER_INTERFACE"

cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: ${MACVLAN_MULTUS_NAME}
  namespace: kube-system
spec:
  cniType: macvlan
  enableCoordinator: true
  macvlan:
    master:
    - ${MACVLAN_MASTER_INTERFACE}
EOF
```

在本文示例中，使用如上配置，创建如下的 Macvlan SpiderMultusConfig，将基于它自动生成的 Multus NetworkAttachmentDefinition CR。

```bash
~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -n kube-system
NAME             AGE
macvlan-ens192   16m

~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system
NAME             AGE
macvlan-ens192   17m
```

### 创建 IPPool

```bash
~# cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-ippool
spec:
  subnet: 10.6.0.0/16
  ips:
    - 10.6.168.101-10.6.168.110
EOF
```

### 创建 Kubevirt VM 应用

以下的示例 Yaml 中， 会创建 1 个 Kubevirt VM 应用 ，其中：

- `ipam.spidernet.io/ippool`：用于指定 Spiderpool 的 IP 池，Spiderpool 会自动在该池中选择一些 IP 与应用形成绑定，实现 Kubevirt VM 应用的 IP 固定效果。

- `v1.multus-cni.io/default-network`：为应用创建一张默认网卡。

```bash
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm
  labels:
    kubevirt.io/vm: vm-cirros
spec:
  runStrategy: Always
  template:
    metadata:
      annotations:
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
      labels:
        kubevirt.io/vm: vm-cirros
    spec:
      domain:
        devices:
          disks:
            - name: containerdisk
              disk:
                bus: virtio
            - name: cloudinitdisk
              disk:
                bus: virtio
          interfaces:
            - name: default
              passt: {}
        resources:
          requests:
            memory: 64M
      networks:
        - name: default
          pod: {}
      volumes:
        - name: containerdisk
          containerDisk:
            image: quay.io/kubevirt/cirros-container-disk-demo
        - name: cloudinitdisk
          cloudInitNoCloud:
            userData: |
              #!/bin/sh
              echo 'printed from cloud-init userdata'
```

最终，在 Kubevirt VM 应用被创建时，Spiderpool 会从指定 IPPool 中随机选择一个 IP 来与应用形成绑定关系。

```bash
~# kubectl get spiderippool
NAME          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
test-ippool   4         10.6.0.0/16   1                    10               false

~# kubectl get po -l vm.kubevirt.io/name=vm-cirros -o wide
NAME                            READY   STATUS      RESTARTS   AGE     IP              NODE                   NOMINATED NODE   READINESS GATES
virt-launcher-vm-cirros-rg6fs   2/2     Running     0          3m43s   10.6.168.105    node2                  <none>           1/1
```

重启 Kubevirt VM Pod, 观察到新的 Pod 的 IP 不会变化，符合预期。

```bash
~# kubectl delete pod virt-launcher-vm-cirros-rg6fs
pod "virt-launcher-vm-cirros-rg6fs" deleted

~# kubectl get po -l vm.kubevirt.io/name=vm-cirros -o wide
NAME                            READY   STATUS      RESTARTS   AGE     IP              NODE                   NOMINATED NODE   READINESS GATES
virt-launcher-vm-cirros-d68l2   2/2     Running     0          1m21s   10.6.168.105    node2                  <none>           1/1
```

重启 Kubevirt VMI，观察到后续新的 Pod 的IP 也不会变化，符合预期。

```bash
~# kubectl delete vmi vm-cirros
virtualmachineinstance.kubevirt.io "vm-cirros" deleted

~# kubectl get po -l vm.kubevirt.io/name=vm-cirros -o wide
NAME                            READY   STATUS    RESTARTS   AGE    IP              NODE                   NOMINATED NODE   READINESS GATES
virt-launcher-vm-cirros-jjgrl   2/2     Running   0          104s   10.6.168.105    node2                  <none>           1/1
```

## 总结

Spiderpool 能保证 Kubevirt VM Pod 在重启、重建场景下，持续获取到相同的 IP 地址。能很好的满足 Kubevirt 虚拟机的固定 IP 需求。
