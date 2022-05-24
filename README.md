
[![Docker Pulls](https://img.shields.io/docker/pulls/itzg/kube-metrics-reporter)](https://hub.docker.com/r/itzg/kube-metrics-reporter)
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/itzg/kube-metrics-reporter)](https://github.com/itzg/kube-metrics-reporter/releases/latest)
[![CircleCI](https://circleci.com/gh/itzg/kube-metrics-reporter.svg?style=svg)](https://circleci.com/gh/itzg/kube-metrics-reporter)

Simple application that accesses the [Kubernetes metrics API](https://github.com/kubernetes/metrics) and reports pod-container metrics.

The Metrics API is exposed by a deployed [Metrics Server](https://kubernetes.io/docs/tasks/debug-application-cluster/resource-metrics-pipeline/#metrics-server) which is included in most managed clusters. [It can also be deployed separately.](https://github.com/kubernetes-sigs/metrics-server).

## Stand-alone Usage

The `kube-metrics-reporter` executable can be executed outside of Kubernetes cluster, in which case it will locate and use the kubernetes configuration from the standard location(s).

```
  -include-labels
    	include pod labels in reported metrics (env INCLUDE_LABELS)
  -interval duration
    	the interval of metrics collection (env INTERVAL)
  -namespace string
    	the namespace of the pods to collect (env NAMESPACE) (default "default")
  -telegraf-endpoint string
    	if configured, metrics will be sent as line protocol to telegraf (env TELEGRAF_ENDPOINT)
```

## In-cluster Usage

With a service account defined with the correct roles, [as described below](#service-account), the reporter can be deployed with a pod manifest such as the following:

```yaml
    metadata:
      name: kube-metrics-reporter
      labels:
        app: kube-metrics-reporter
    spec:
      serviceAccountName: kube-metrics-monitor
      containers:
        - name: kube-metrics-reporter
          image: itzg/kube-metrics-reporter
          env:
            - name: TELEGRAF_ENDPOINT
              value: telegraf:8094
            - name: INTERVAL
              value: "1m"
            - name: INCLUDE_LABELS
              value: "true"
            - name: NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
```

> The example assumes a telegraf service in the same namespace with a socket_listener input plugin configured for port 8094.

## Reporters

**NOTE** the units reported match that of `kubectl top pods` where
- CPU usage is reported in millicores, which is 1/1000th of a vCPU core
- memory usage is reported in [mebibytes](https://en.wikipedia.org/wiki/Mebibyte) (Mi).

### Console

By default, metrics are reported to the console, such as:

```
2019-12-27T22:39:36-06:00 pod=grafana-0, container=grafana, cpu=1m, mem=20Mi
2019-12-27T22:39:36-06:00 pod=nginx-ingress-controller-857f44797-gs92j, container=nginx-ingress-controller, cpu=6m, mem=111Mi
2019-12-27T22:39:36-06:00 pod=telegraf-mwrh9, container=telegraf, cpu=1m, mem=22Mi
2019-12-27T22:39:36-06:00 pod=influxdb-0, container=influxdb, cpu=2m, mem=37Mi
```

If an interval is given, then the application will continue to run reporting metrics at the given interval. 

### Telegraf

When the telegraf endpoint is configured, the metrics will be sent using Influx line protocol to the `host:port` given. The endpoint should be a socket_listener plugin configured such as:

```toml
[[inputs.socket_listener]]
  service_address = "tcp://:8094"
```

The reported metrics will look like the following:
```
kubernetes_pod_container,container_name=nginx-ingress-controller,host=dbc5f9812889,namespace=default,pod_name=nginx-ingress-controller-857f44797-gs92j cpu_usage_millicores=8i,memory_usage_mbytes=111i 1577507390268680300
kubernetes_pod_container,container_name=grafana,host=dbc5f9812889,namespace=default,pod_name=grafana-0 cpu_usage_millicores=1i,memory_usage_mbytes=20i 1577507390268680300
kubernetes_pod_container,container_name=influxdb,host=dbc5f9812889,namespace=default,pod_name=influxdb-0 cpu_usage_millicores=1i,memory_usage_mbytes=37i 1577507390268680300
```

If labels are included, they are conveyed as tags with the prefix "label_".

## Service account

Since this application accesses the metrics API of the kubernetes API service, the pod will need to be assigned a service account with an appropriate role. 

> Service accounts must be present before the deployment, so either ensure the service account manifest is applied first or place the service account yaml documents before the deployment in the same manifest file.

The following shows how a service account could be declared:

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kube-metrics-monitor
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kube-metrics-monitor
rules:
  - apiGroups: ["metrics.k8s.io"]
    resources:
      - pods
    verbs: ["get", "list"]
  - apiGroups: [""]
    resources:
      - pods
    verbs: ["list","watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kube-metrics-monitor
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kube-metrics-monitor
subjects:
  - kind: ServiceAccount
    name: kube-metrics-monitor
```

> If not including labels, you can remove the pods watch on `apiGroups:[""]`
