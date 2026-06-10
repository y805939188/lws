# Enabling Gang Scheduling with different Schedulers

This document provides guidance about how to enable gang scheduling capabilities in LeaderWorkerSet with different schedulers.

## Supported Schedulers

| Custom Scheduler Community                   | GitHub ID         | Email                           |
| -------------------------------------------- | ----------------- | ------------------------------- |
| Volcano (`@volcano-sh`)                      | `@JesseStutler`   | `jesseincomparable@hotmail.com` |
| ACK Koordinator (Moonshot fork, label-only)  | Moonshot internal | N/A                             |
| Your-Scheduler-Here                          | `@your-id`        | `your@email.com`                |

We welcome integrations for more custom schedulers that support gang scheduling capabilities. Getting started with:

1.  Open an issue on the [LeaderWorkerSet GitHub repository](https://github.com/kubernetes-sigs/lws/issues) to discuss your proposal, tagging the LWS maintainers.
2.  In the same pull request that adds the scheduler integration, update the maintainer matrix above with your contact information. This can help us contact the maintainer to regularly upgrade the custom scheduler dependencies.

## Using Volcano for Gang Scheduling

### Installation

To enable gang scheduling, you must enable the feature flag and specify `volcano` as the provider during the LeaderWorkerSet controller installation.

- **With Helm:**

  Refer to the [install-by-helm](https://lws.sigs.k8s.io/docs/installation/#install-by-helm). When installing, add the following flags to your `helm install` command:
  ```sh
  helm install lws oci://ghcr.io/kubernetes-sigs/lws-charts/lws \
    --set gangSchedulingManagement.schedulerProvider=volcano
  ```

- **With kubectl:**

  Refer to the [install-by-kubectl](https://lws.sigs.k8s.io/docs/installation/#install-by-kubectl). Gang scheduling is **disabled by default**. To enable gang scheduling capabilities, you need to:

  1. **Update the configuration ConfigMap** to enable gang scheduling settings:
     ```yaml
     apiVersion: v1
     kind: ConfigMap
     metadata:
       name: lws-manager-config
       namespace: lws-system
     data:
       controller_manager_config.yaml: |
         apiVersion: config.lws.x-k8s.io/v1alpha1
         kind: Configuration
         leaderElection:
           leaderElect: true
         internalCertManagement:
           enable: true
         # Add gang scheduling configuration
         gangSchedulingManagement:
           schedulerProvider: volcano
     ```

  2. **Enable the Volcano RBAC component** to grant the LWS controller permission to manage Volcano PodGroups. In `config/default/kustomization.yaml`, uncomment the `components` section:
     ```yaml
     # [VOLCANO] To enable Volcano gang scheduling RBAC, uncomment the following.
     components:
     - ../components/volcano
     ```
     Then re-apply the manifests:
     ```sh
     kubectl apply -k config/default
     ```

  3. **Restart the lws-controller-manager** to apply the new configuration:
     ```sh
     kubectl rollout restart deployment/lws-controller-manager -n lws-system
     ```

     Or you can directly delete the pod to trigger a restart:
     ```sh
     kubectl delete pod -l control-plane=controller-manager -n lws-system
     ```

### Startup Policy Differences

The `startupPolicy` in your LeaderWorkerSet spec determines the gang scheduling behavior by setting the `minMember` of the auto-generated `PodGroup`.

- **`LeaderCreated` Policy (Default):** `MinMember` is set to the full replica size (1 leader + (size-1) workers). All pods in the replica must be scheduled together.
- **`LeaderReady` Policy:** `MinMember` is set to 1, therefore the leader pod can be scheduled immediately and won't be blocked by the gang scheduler. 
However, the `minResources` required by the `PodGroup` will still be calculated based on the entire group. 
This ensures that resources for all pods are reserved before the leader starts, guaranteeing that the workers can eventually be scheduled.

### Specifying a Custom Queue

To assign all groups of a LeaderWorkerSet to a specific Volcano queue, add the `scheduling.volcano.sh/queue-name` annotation to the LeaderWorkerSet metadata.


### Volcano Scheduler Configurations

To configure certain annotations in the PodGroup that are used by the Volcano scheduler to control specific scheduling behaviors, the PodGroup created by the LeaderWorkerSet will inherit the annotations starting with `volcano.sh/` from the LeaderWorkerSet's metadata.annotations.

For a complete example, please refer to the [lws-sample-volcano.yaml](./lws-sample-volcano.yaml) file.

## 使用 ACK Koordinator 进行 Gang Scheduling

ACK Koordinator 的 v1 接入使用 ACK kube-scheduler 支持的 label-only gang scheduling 模式。LWS 不会创建或管理 PodGroup CR，也不需要新增 RBAC、scheme 注册或额外的调度器客户端；webhook 只会在 opt-in 的 Pod 上注入 ACK gang labels：

- `pod-group.scheduling.sigs.k8s.io/name`
- `pod-group.scheduling.sigs.k8s.io/min-available`

### 启用方式

Koordinator provider 需要两级开关同时满足：

1. 部署 LWS controller 时启用全局 provider：

   ```sh
   helm install lws oci://ghcr.io/kubernetes-sigs/lws-charts/lws \
     --set gangSchedulingManagement.schedulerProvider=koordinator
   ```

   如果使用 kubectl 安装，需要在 `lws-manager-config` 的 `controller_manager_config.yaml` 中配置：

   ```yaml
   gangSchedulingManagement:
     schedulerProvider: koordinator
   ```

   controller 镜像需要包含 Koordinator provider 代码，并且应与这项配置同批升级。旧镜像通常无法识别 `koordinator` provider；如果只给 LWS 对象添加 opt-in label，但全局 provider 没有启用，webhook 不会注入 ACK gang labels。

2. 在需要使用 ACK Koordinator gang scheduling 的 LeaderWorkerSet metadata labels 上添加 opt-in label：

   ```yaml
   apiVersion: leaderworkerset.x-k8s.io/v1
   kind: LeaderWorkerSet
   metadata:
     name: sample
     labels:
       lws.moonshot.io/gang-scheduler-provider: koordinator
   spec:
     startupPolicy: LeaderCreated
     # ...
   ```

   这个 label 只允许写在 LeaderWorkerSet 的 `metadata.labels` 上。不要写到 `leaderTemplate` 或 `workerTemplate` 的 labels 中，admission webhook 会拒绝这种配置。

### 行为边界

- `lws.moonshot.io/gang-scheduler-provider=koordinator` 只在全局 `schedulerProvider: koordinator` 时生效。如果全局 provider 是 `volcano`，这个 opt-in label 会被忽略，Pod 仍按 Volcano provider 的逻辑注入 `scheduling.k8s.io/group-name` annotation。
- Koordinator provider 不会创建 PodGroup CR。ACK kube-scheduler 会根据 Pod 上的 ACK gang labels 处理 gang 关系，因此不要依赖 `kubectl get podgroup` 来确认 LWS 是否创建了 PodGroup 对象。
- v1 只支持 `startupPolicy: LeaderCreated`。`LeaderReady` 会被 admission webhook 拒绝，因为 leader 需要先 Ready 才会创建 worker，而 ACK label-only gang 又要求 `min-available` 个 Pod 同时等待调度，这会形成确定性死锁。
- opt-in label 创建后不可变。需要开启、关闭或切换 provider 时，请重建 LeaderWorkerSet，避免绕过 LWS 自身 revision/rollout 机制触发不受控的 Pod 重建。
- provider 不会设置 `schedulerName`。ACK 托管集群通常由默认 kube-scheduler 提供 gang 能力，不需要在 Pod 模板里设置 `schedulerName`；自建开源 Koordinator 环境如需使用 `koord-scheduler`，应由用户在 leader/worker 模板中显式配置。
- opt-in 的 LeaderWorkerSet 不要手动设置 `pod-group.scheduling.sigs.k8s.io/name` 或 `pod-group.scheduling.sigs.k8s.io/min-available`，provider 会按 LWS 的 group index、revision 和 size 覆盖它们。非 opt-in 的 LeaderWorkerSet 如果手写这些 ACK labels，LWS 会原样透传。
- DisaggregatedSet 与 Koordinator provider 的组合暂未验证，v1 不声明支持。

### 后续验证项

ACK label-only gang scheduling 的单 Pod 重建语义需要在真实 ACK 集群中继续验证。建议在 PoC 中覆盖以下场景：

- 删除运行中的 opt-in 组内单个 worker，观察重建 Pod 是否能重新调度。
- 分别验证默认的 `restartPolicy: RecreateGroupOnPodRestart` 和 `restartPolicy: None`。
- 如发现单 Pod 重建在 gang 等待阶段卡住，再评估是否需要显式注入 ACK 的 `match-policy` label，例如 `once-satisfied`。
