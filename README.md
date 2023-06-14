**NOTE**: This repository contains code and example custom resources for the Kubernetes Operator for YugabyteDB v0.0.1. **This operator is no longer maintained.** We recommend using the actively maintained [helm charts for YugabyteDB](https://docs.yugabyte.com/preview/deploy/kubernetes/single-zone/oss/helm-chart/) to install YugabyteDB on Kubernetes.

# YugabyteDB Kubernetes Operator (legacy)

Deploy and manage YugabyteDB cluster as a kubernetes native custom resource viz. `ybclusters.yugabyte.com`.

## Deploy a YugabyteDB cluster with this operator

To create a YugabyteDB cluster, first we need to register the custom resource that would represent YugabyteDB cluster: `ybclusters.yugabyte.com`.

```sh
kubectl create -f deploy/crds/yugabyte.com_ybclusters_crd.yaml
```

Setup RBAC for operator and create the operator itself. Run the following command, from root of the repository, to do the same.

```sh
kubectl create -f deploy/operator.yaml
```

After a few seconds the operator should be up & running. Verify the operator status by running below command.
```sh
kubectl -n yb-operator get po,deployment
```

Register the custom resource that would represent YugabyteDB cluster, i.e. `ybclusters.yugabyte.com`.
```sh
kubectl create -f deploy/crds/yugabyte.com_ybclusters_crd.yaml
```

Finally create an instance of the custom resource with which the operator would create a YugabyteDB cluster. The sample manifest has following content,
```yaml
apiVersion: yugabyte.com/v1alpha1
kind: YBCluster
metadata:
  name: example-ybcluster
  namespace: yb-operator
spec:
  replicationFactor: 3
  domain: cluster.local
  master:
    replicas: 3
    storage:
      size: 1Gi
  tserver:
    replicas: 3
    storage:
      count: 1
      size: 1Gi
```
```sh
kubectl create -f deploy/crds/yugabyte.com_v1alpha1_ybcluster_cr.yaml
```

Verify that the cluster is up and running with below command. You should see 3 pods each for YB-Master & YB-TServer.
```sh
kubectl get po,sts,svc
```
Once the cluster is up and running, you may start the postgres-compatible ysql api & start executing relational queries.
```sh
kubectl exec -it yb-tserver-0 /home/yugabyte/bin/ysqlsh -- -h yb-tserver-0 --echo-queries
```
You may choose to start cassandra-compatible ycql api and start storing data in NoSql format.
```sh
kubectl exec -it yb-tserver-0 /home/yugabyte/bin/cqlsh yb-tserver-0
```

You can read more about ysql and ycql apis in [YugabyteDB documentation](https://docs.yugabyte.com/latest/api/)

## Configuration options

### Image
Mention YugabyteDB docker image attributes such as `repository`, `tag` and `pullPolicy` under `image`.


### Replication Factor
Specify the required data replication factor. This is a **required** field.

### Master/TServer
Master & TServer are two essential components of a YugabyteDB cluster. Master is responsible for recording and maintaining system metadata & for admin activities. TServers are mainly responsible for data I/O.
Specify Master/TServer specific attributes under `master`/`tserver`. The valid attributes are as described below. These two are **required** fields.

#### Replicas
Specify count of pods for `master` & `tserver` under `replicas` field. This is a **required** field.

#### Network Ports
Control network configuration for Master & TServer, each of which support only a selected port attributes. Below table depicts the supported port attributes.
Note that these are **optional** fields, except `tserver.tserverUIPort`, hence below table also mentions default values for each port. Default network configuration will be used, if any or all of the acceptable fields are absent.

A ClusterIP service will be created when `tserver.tserverUIPort` port is specified. If it is not specified, only StatefulSet & headless service will be created for TServer. ClusterIP service creation will be skipped. Whereas for Master, all 3 kubernetes objects will always be created.

If `master.enableLoadBalancer` is set to `true`, then master UI service will be of type `LoadBalancer`. TServer UI service will be of type `LoadBalancer`, if `tserver.tserverUIPort` is specified and `tserver.enableLoadBalancer` is set to `true`. `tserver.enableLoadBalancer` will be ignored if `tserver.tserverUIPort` is not specified.

Table depicting acceptable port names, applicable component (Master/TServer) and port default values:

| Attribute      | Component | Default Value |
| -------------- | --------- | ------------- |
| masterUIPort   | Master    | 7000          |
| masterRPCPort  | Master    | 7100          |
| tserverUIPort  | TServer   | NA            |
| tserverRPCPort | TServer   | 9100          |
| ycqlPort       | TServer   | 9042          |
| yedisPort      | TServer   | 6379          |
| ysqlPort       | TServer   | 5433          |

#### podManagementPolicy
Specify pod management policy for statefulsets created as part of YugabyteDB cluster. Valid values are `Parallel` & `OrderedReady`, `Parallel` being the default value.

#### storage
Specify storage configurations viz. Storage `count`, `size` & `storageClass` of volumes. Typically 1 volume per Master instance is sufficient, hence Master has a default storage count of `1`. If storage class isn't specified, it will be defaulted to `standard`. Make sure kubernetes admin has defined `standard` storage class, before leaving this field out.

#### resources
Specify resource `requests` & `limits` under `resources` attribute. The resources to be specified are `cpu` & `memory`. The `resource` property in itself is optional & it won't be applied to created `StatefulSets`, if omitted. You may also choose to specify either `resource.requests` or `resource.limits` or both.

#### gflags
Specify list of GFlags for additional control on YugabyteDB cluster. Refer [Master Config Flags](https://docs.yugabyte.com/latest/admin/yb-master/#config-flags) & [TServer Config Flags](https://docs.yugabyte.com/latest/admin/yb-tserver/#config-flags) for list of supported flags.

If you have enabled TLS encryption, then you can set:
- `use_node_to_node_encryption` flag to enable node to node encryption
- `allow_insecure_connections` flag to specify if insecure connections are allowed when tls is enabled
- `use_client_to_server_encryption` flag to enable client to node encryption
