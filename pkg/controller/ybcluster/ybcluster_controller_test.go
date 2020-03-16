package ybcluster

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	ybv1alpha1 "github.com/yugabyte/yugabyte-k8s-operator/pkg/apis/yugabyte/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestValidate(t *testing.T) {
	minimalCluster := getMinimalClusterSpec()
	fullCluster := getFullClusterSpec()

	err := validateCR(minimalCluster)
	assert.Nil(t, err, fmt.Sprintf("received error: %+v", err))

	err = validateCR(fullCluster)
	assert.Nil(t, err)
}

func TestAddDefaults(t *testing.T) {
	minimalCluster := getMinimalClusterSpec()
	addDefaults(minimalCluster)

	// Validate Image attributes
	assert.NotNil(t, &minimalCluster.Image)
	assert.NotNil(t, &minimalCluster.Image.Repository)
	assert.Equal(t, imageRepositoryDefault, minimalCluster.Image.Repository)
	assert.NotNil(t, &minimalCluster.Image.Tag)
	assert.Equal(t, imageTagDefault, minimalCluster.Image.Tag)
	assert.NotNil(t, &minimalCluster.Image.PullPolicy)
	assert.Equal(t, imagePullPolicyDefault, minimalCluster.Image.PullPolicy)

	// Validate TLS attributes
	assert.NotNil(t, &minimalCluster.TLS)
	assert.NotNil(t, &minimalCluster.TLS.Enabled)
	assert.False(t, minimalCluster.TLS.Enabled)

	// Validate ReplicationFactor
	assert.NotNil(t, &minimalCluster.ReplicationFactor)
	assert.Equal(t, getMinimalClusterSpec().ReplicationFactor, minimalCluster.ReplicationFactor)

	// Validate Master attributes
	assert.NotNil(t, &minimalCluster.Master)
	assert.NotNil(t, &minimalCluster.Master.Replicas)
	assert.Equal(t, getMinimalClusterSpec().Master.Replicas, minimalCluster.Master.Replicas)
	assert.NotNil(t, &minimalCluster.Master.MasterUIPort)
	assert.Equal(t, masterUIPortDefault, minimalCluster.Master.MasterUIPort)
	assert.NotNil(t, &minimalCluster.Master.MasterRPCPort)
	assert.Equal(t, masterRPCPortDefault, minimalCluster.Master.MasterRPCPort)
	assert.NotNil(t, &minimalCluster.Master.EnableLoadBalancer)
	assert.False(t, minimalCluster.Master.EnableLoadBalancer)
	assert.NotNil(t, &minimalCluster.Master.PodManagementPolicy)
	assert.Equal(t, podManagementPolicyDefault, string(minimalCluster.Master.PodManagementPolicy))
	assert.NotNil(t, &minimalCluster.Master.Storage)
	assert.NotNil(t, &minimalCluster.Master.Storage.Count)
	assert.Equal(t, storageCountDefault, minimalCluster.Master.Storage.Count)
	assert.NotNil(t, &minimalCluster.Master.Storage.StorageClass)
	assert.Equal(t, storageClassDefault, minimalCluster.Master.Storage.StorageClass)

	// Validate Tserver attributes
	assert.NotNil(t, &minimalCluster.Tserver)
	assert.NotNil(t, &minimalCluster.Tserver.Replicas)
	assert.Equal(t, getMinimalClusterSpec().Tserver.Replicas, minimalCluster.Tserver.Replicas)
	assert.NotNil(t, &minimalCluster.Tserver.TserverUIPort)
	assert.Equal(t, getMinimalClusterSpec().Tserver.TserverUIPort, minimalCluster.Tserver.TserverUIPort)
	assert.NotNil(t, &minimalCluster.Tserver.TserverRPCPort)
	assert.Equal(t, tserverRPCPortDefault, minimalCluster.Tserver.TserverRPCPort)
	assert.NotNil(t, &minimalCluster.Tserver.YCQLPort)
	assert.Equal(t, ycqlPortDefault, minimalCluster.Tserver.YCQLPort)
	assert.NotNil(t, &minimalCluster.Tserver.YedisPort)
	assert.Equal(t, yedisPortDefault, minimalCluster.Tserver.YedisPort)
	assert.NotNil(t, &minimalCluster.Tserver.YSQLPort)
	assert.Equal(t, ysqlPortDefault, minimalCluster.Tserver.YSQLPort)
	assert.NotNil(t, &minimalCluster.Tserver.EnableLoadBalancer)
	assert.False(t, minimalCluster.Tserver.EnableLoadBalancer)
	assert.NotNil(t, &minimalCluster.Tserver.PodManagementPolicy)
	assert.Equal(t, podManagementPolicyDefault, string(minimalCluster.Tserver.PodManagementPolicy))
	assert.NotNil(t, &minimalCluster.Tserver.Storage)
	assert.NotNil(t, &minimalCluster.Tserver.Storage.Count)
	assert.Equal(t, storageCountDefault, minimalCluster.Tserver.Storage.Count)
	assert.NotNil(t, &minimalCluster.Tserver.Storage.Size)
	assert.NotNil(t, &minimalCluster.Tserver.Storage.StorageClass)
	assert.Equal(t, storageClassDefault, minimalCluster.Tserver.Storage.StorageClass)
}

func TestCreateAppLabels(t *testing.T) {
	labelStr := "my-label"
	labelMap := createAppLabels(labelStr)

	assert.Equal(t, labelStr, labelMap[appLabel])
}

func TestCreateServicePorts(t *testing.T) {
	minimalCluster := getMinimalClusterSpec()
	addDefaults(minimalCluster)
	masterServicePorts := createServicePorts(minimalCluster, false)

	assert.Equal(t, 2, len(masterServicePorts))
	assert.Equal(t, uiPortName, masterServicePorts[0].Name)
	assert.Equal(t, masterUIPortDefault, masterServicePorts[0].Port)
	assert.Equal(t, rpcPortName, masterServicePorts[1].Name)
	assert.Equal(t, masterRPCPortDefault, masterServicePorts[1].Port)

	tserverServicePorts := createServicePorts(minimalCluster, true)

	assert.Equal(t, 4, len(tserverServicePorts))
	assert.Equal(t, rpcPortName, tserverServicePorts[0].Name)
	assert.Equal(t, tserverRPCPortDefault, tserverServicePorts[0].Port)
	assert.Equal(t, ycqlPortName, tserverServicePorts[1].Name)
	assert.Equal(t, ycqlPortDefault, tserverServicePorts[1].Port)
	assert.Equal(t, yedisPortName, tserverServicePorts[2].Name)
	assert.Equal(t, yedisPortDefault, tserverServicePorts[2].Port)
	assert.Equal(t, ysqlPortName, tserverServicePorts[3].Name)
	assert.Equal(t, ysqlPortDefault, tserverServicePorts[3].Port)

	minimalCluster.Tserver.TserverUIPort = 9000
	tserverServicePorts = createServicePorts(minimalCluster, true)

	assert.Equal(t, 5, len(tserverServicePorts))
	assert.Equal(t, uiPortName, tserverServicePorts[4].Name)
	assert.Equal(t, minimalCluster.Tserver.TserverUIPort, tserverServicePorts[4].Port)
}

func TestCreateUIServicePorts(t *testing.T) {
	minimalCluster := getMinimalClusterSpec()
	addDefaults(minimalCluster)

	masterUIPorts := createUIServicePorts(minimalCluster, false)

	assert.Equal(t, 1, len(masterUIPorts))
	assert.Equal(t, uiPortName, masterUIPorts[0].Name)
	assert.Equal(t, masterUIPortDefault, masterUIPorts[0].Port)

	tserverUIPorts := createUIServicePorts(minimalCluster, true)

	assert.Nil(t, tserverUIPorts)

	minimalCluster.Tserver.TserverUIPort = 9000
	tserverUIPorts = createUIServicePorts(minimalCluster, true)

	assert.NotNil(t, tserverUIPorts)
	assert.Equal(t, 1, len(tserverUIPorts))
	assert.Equal(t, uiPortName, tserverUIPorts[0].Name)
	assert.Equal(t, minimalCluster.Tserver.TserverUIPort, tserverUIPorts[0].Port)
}

func TestCreateMasterContainerCommand(t *testing.T) {
	minimalCluster := getMinimalClusterSpec()
	addDefaults(minimalCluster)

	masterCommand := createMasterContainerCommand(
		"default",
		minimalCluster.Master.MasterRPCPort,
		minimalCluster.Master.Replicas,
		minimalCluster.ReplicationFactor,
		minimalCluster.Master.Storage.Count,
		minimalCluster.Master.Gflags,
		minimalCluster.TLS.Enabled,
	)

	assert.Equal(t, 11, len(masterCommand))

	minimalCluster.Master.Gflags = []ybv1alpha1.YBGFlagSpec{
		{
			Key:   "default_memory_limit_to_ram_ratio",
			Value: "0.85",
		},
	}
	masterCommand = createMasterContainerCommand(
		"default",
		minimalCluster.Master.MasterRPCPort,
		minimalCluster.Master.Replicas,
		minimalCluster.ReplicationFactor,
		minimalCluster.Master.Storage.Count,
		minimalCluster.Master.Gflags,
		minimalCluster.TLS.Enabled,
	)

	assert.Equal(t, 12, len(masterCommand))
}

func TestCreateTServerContainerCommand(t *testing.T) {
	minimalCluster := getMinimalClusterSpec()
	addDefaults(minimalCluster)

	tserverCommand := createTServerContainerCommand(
		"default",
		minimalCluster.Master.MasterRPCPort,
		minimalCluster.Tserver.TserverRPCPort,
		minimalCluster.Tserver.YSQLPort,
		minimalCluster.Master.Replicas,
		minimalCluster.Tserver.Storage.Count,
		minimalCluster.Tserver.Gflags,
		minimalCluster.TLS.Enabled,
	)

	assert.Equal(t, 11, len(tserverCommand))

	minimalCluster.Tserver.Gflags = []ybv1alpha1.YBGFlagSpec{
		{
			Key:   "default_memory_limit_to_ram_ratio",
			Value: "0.85",
		},
	}
	tserverCommand = createTServerContainerCommand(
		"default",
		minimalCluster.Master.MasterRPCPort,
		minimalCluster.Tserver.TserverRPCPort,
		minimalCluster.Tserver.YSQLPort,
		minimalCluster.Master.Replicas,
		minimalCluster.Tserver.Storage.Count,
		minimalCluster.Tserver.Gflags,
		minimalCluster.TLS.Enabled,
	)

	assert.Equal(t, 12, len(tserverCommand))
}

func TestCreateListOfVolumeMountPaths(t *testing.T) {
	expectedVolumeMountPaths := "/mnt/data0,/mnt/data1,/mnt/data2"
	volumeMountPaths := createListOfVolumeMountPaths(3)
	assert.Equal(t, expectedVolumeMountPaths, volumeMountPaths)
}

func TestReconcile(t *testing.T) {
	name := "cluster"
	namespace := "cluster-namespace"
	minimalCluster := &ybv1alpha1.YBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: *getMinimalClusterSpec(),
	}
	objs := []runtime.Object{minimalCluster}
	s := scheme.Scheme
	s.AddKnownTypes(ybv1alpha1.SchemeGroupVersion, minimalCluster)
	cl := fake.NewFakeClient(objs...)
	r := &ReconcileYBCluster{client: cl, scheme: s}
	recReq := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	res, err := r.Reconcile(recReq)

	assert.Nil(t, err)
	assert.NotNil(t, res)

	service := &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: masterNamePlural, Namespace: namespace}, service)
	assert.Nil(t, err)
	assert.Equal(t, masterNamePlural, service.Name)

	service = &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: tserverNamePlural, Namespace: namespace}, service)
	assert.Nil(t, err)
	assert.Equal(t, tserverNamePlural, service.Name)

	service = &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: masterUIServiceName, Namespace: namespace}, service)
	assert.Nil(t, err)
	assert.Equal(t, masterUIServiceName, service.Name)

	service = &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: tserverUIServiceName, Namespace: namespace}, service)
	assert.True(t, errors.IsNotFound(err))
	assert.Empty(t, service.Name)

	sfs := &appsv1.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: masterName, Namespace: namespace}, sfs)
	assert.Nil(t, err)
	assert.Equal(t, masterName, sfs.Name)
	assert.Equal(t, getMinimalClusterSpec().Master.Replicas, *sfs.Spec.Replicas)

	sfs = &appsv1.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: tserverName, Namespace: namespace}, sfs)
	assert.Nil(t, err)
	assert.Equal(t, tserverName, sfs.Name)
	assert.Equal(t, getMinimalClusterSpec().Tserver.Replicas, *sfs.Spec.Replicas)
}

func TestReconcileWithTLS(t *testing.T) {
	name := "cluster"
	namespace := "cluster-namespace"
	minimalCluster := &ybv1alpha1.YBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: *getFullClusterSpec(),
	}
	objs := []runtime.Object{minimalCluster}
	s := scheme.Scheme
	s.AddKnownTypes(ybv1alpha1.SchemeGroupVersion, minimalCluster)
	cl := fake.NewFakeClient(objs...)
	r := &ReconcileYBCluster{client: cl, scheme: s}
	recReq := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	res, err := r.Reconcile(recReq)

	assert.Nil(t, err)
	assert.NotNil(t, res)

	service := &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: masterNamePlural, Namespace: namespace}, service)
	assert.Nil(t, err)
	assert.Equal(t, masterNamePlural, service.Name)

	service = &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: tserverNamePlural, Namespace: namespace}, service)
	assert.Nil(t, err)
	assert.Equal(t, tserverNamePlural, service.Name)

	service = &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: masterUIServiceName, Namespace: namespace}, service)
	assert.Nil(t, err)
	assert.Equal(t, masterUIServiceName, service.Name)

	service = &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: tserverUIServiceName, Namespace: namespace}, service)
	assert.Nil(t, err)
	assert.Equal(t, tserverUIServiceName, service.Name)

	sfs := &appsv1.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: masterName, Namespace: namespace}, sfs)
	assert.Nil(t, err)
	assert.Equal(t, masterName, sfs.Name)
	assert.Equal(t, getFullClusterSpec().Master.Replicas, *sfs.Spec.Replicas)
	assert.Equal(t, strings.Join([]string{getFullClusterSpec().Image.Repository, getFullClusterSpec().Image.Tag}, ":"), sfs.Spec.Template.Spec.Containers[0].Image)

	sfs = &appsv1.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: tserverName, Namespace: namespace}, sfs)
	assert.Nil(t, err)
	assert.Equal(t, tserverName, sfs.Name)
	assert.Equal(t, getFullClusterSpec().Tserver.Replicas, *sfs.Spec.Replicas)
}

func TestGetMasterAddresses(t *testing.T) {
	expected := "yb-master-0.yb-masters.yb-operator.svc.cluster.local:7100,yb-master-1.yb-masters.yb-operator.svc.cluster.local:7100,yb-master-2.yb-masters.yb-operator.svc.cluster.local:7100"

	actual := getMasterAddresses("yb-operator", int32(7100), int32(3))

	assert.Equal(t, expected, actual)
}

func getMinimalClusterSpec() *ybv1alpha1.YBClusterSpec {
	return &ybv1alpha1.YBClusterSpec{
		ReplicationFactor: 3,
		Master: ybv1alpha1.YBMasterSpec{
			Replicas: 3,
			Storage: ybv1alpha1.YBStorageSpec{
				Size: "1Gi",
			},
		},
		Tserver: ybv1alpha1.YBTServerSpec{
			Replicas: 3,
			Storage: ybv1alpha1.YBStorageSpec{
				Size: "10Gi",
			},
		},
	}
}

func getFullClusterSpec() *ybv1alpha1.YBClusterSpec {
	return &ybv1alpha1.YBClusterSpec{
		Image: ybv1alpha1.YBImageSpec{
			Repository: imageRepositoryDefault,
			Tag:        "1.3.0.0-b1",
			PullPolicy: imagePullPolicyDefault,
		},
		TLS: ybv1alpha1.YBTLSSpec{
			Enabled: true,
			RootCA: ybv1alpha1.YBRootCASpec{
				Cert: "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURGRENDQWZ5Z0F3SUJBZ0lVUk9vY2RNbUxZU2YwOWRkQTBPa0VWODJtTTRVd0RRWUpLb1pJaHZjTkFRRUwKQlFBd01ERVJNQThHQTFVRUNnd0lXWFZuWVdKNWRHVXhHekFaQmdOVkJBTU1Fa05CSUdadmNpQlpkV2RoWW5sMApaU0JFUWpBZUZ3MHhPVEE1TWpNd09URXhNalphRncweE9URXdNak13T1RFeE1qWmFNREF4RVRBUEJnTlZCQW9NCkNGbDFaMkZpZVhSbE1Sc3dHUVlEVlFRRERCSkRRU0JtYjNJZ1dYVm5ZV0o1ZEdVZ1JFSXdnZ0VpTUEwR0NTcUcKU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQ3hMd3hMeTZjYjhZdS9laFlmU3pteWVqcEw1cjZVSnFmeApWVDF5SUJWSktzNnZPR0QzRWdhSzZaVVZkODBzOGdZU3hzbG1UREh0cnNyWXhCSVMwRXJMVFFsODZVUVd5d1lsCjkvT09FYzdGUVRoWmdsZHRDQmwxNUZSSmp4TGk0LzFJUU1xaU93TDlGM0xjQ3JobXF1bVZKWXJmVGRtNnp4YXkKY0Q3ZlNISGw5Y3phYWFGUGRRa2JZaUt0V0p2cmJ1WEx2QkhleE0rL2FhaVIwY2FBTE94RkhKRlE0N2E1cWwvVQpOcld6emJuVjFZTkRGcEJmLzY2cm95WFh5L2xGMmhWWStYampudlRNL2M0bURWRHRRcDVKVXR6UGhGZ3ZhQngyClUvL2paeFBPblc5dVU4THdFeVdzeWhPNTNWcEREN3VMSHFHYnFrZWd0OVBtTmZsT1QwMlhBZ01CQUFHakpqQWsKTUE0R0ExVWREd0VCL3dRRUF3SUM1REFTQmdOVkhSTUJBZjhFQ0RBR0FRSC9BZ0VCTUEwR0NTcUdTSWIzRFFFQgpDd1VBQTRJQkFRQitXS0NlS3JiYXkwaFNMdjMreUlEWEkvaEw5TEY1N3pDUzNtRU1JTEJXcUVXbUN1R25WVXNFCktSMXA0VzBPSERXbStzTWNtbm1EcUM4U1VZMHlOM2htYnVGWWViZUs5bks0R3IzQ0VtTGk2Q3dxbzJtcWZLVHkKSk9sSU5KZDFFdW1jSlVBUEtmVGNxVkdSWlpITFlHMTVsckpyNjNMaTlYbHhqajlNNEpCejJnd2dsQ3Bqa1BxMwpSazgvYnZSbHhaNVdlNVZ1RUtTckZRNForditkSS9peFpwdjQ3UHY1TTM3bHdLRlQ5amRxbW8zbE82ODNFcEljCnF6bStrclUyb2VhZUphVUFnWlRFU296VUZGb053QzUyTjR3YWlndXo0TU5pa3ZIYWtiTEpEdW9zQTRhQTBnN0sKb2s3K2UvKzlWY0hEUllSaFN5VVhkanB2SmdyVUEyeVYKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=",
				Key:  "LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBc1M4TVM4dW5HL0dMdjNvV0gwczVzbm82UythK2xDYW44VlU5Y2lBVlNTck9yemhnCjl4SUdpdW1WRlhmTkxQSUdFc2JKWmt3eDdhN0syTVFTRXRCS3kwMEpmT2xFRnNzR0pmZnpqaEhPeFVFNFdZSlgKYlFnWmRlUlVTWThTNHVQOVNFREtvanNDL1JkeTNBcTRacXJwbFNXSzMwM1p1czhXc25BKzMwaHg1ZlhNMm1taApUM1VKRzJJaXJWaWI2MjdseTd3UjNzVFB2Mm1va2RIR2dDenNSUnlSVU9PMnVhcGYxRGExczgyNTFkV0RReGFRClgvK3VxNk1sMTh2NVJkb1ZXUGw0NDU3MHpQM09KZzFRN1VLZVNWTGN6NFJZTDJnY2RsUC80MmNUenAxdmJsUEMKOEJNbHJNb1R1ZDFhUXcrN2l4NmhtNnBIb0xmVDVqWDVUazlObHdJREFRQUJBb0lCQUJwbEd0elR1dEpEMm9Dcwp6RXpmSlBvOGtTQ3JnQ3FMMDZyMCt0RmNqQzg1TEU4WUJBSHFjb1VSSlA5c3VHa0FxUHoxRmgyaUxqSHRQeFNwCnFOT2FxZm05UVRPVmdHb3cxbnFqaEduZXAwSGxaR0taTXpMdjZQTVNENmhob3Z1ZjRTUjVXblp1ZWhTQUFNRmMKNjNtSDdvSWtkSnF0ZTBrRC9xcVlaQlZaTW5hQ053cGpsQ0dLYmZLbjlDTHpGSUxhckxiNm4zR2x6dmpRTC9nTAphd3VnNlh5WThvcTlEVjBrZkpEV0lVZFRIc1lFYXFHcGNFWkwvNWZPSlVVcTdSSzV2eWxTVmUrWFZ5OE5jZm1sCk4rQUMrcGNwVFRRR2V0SXZsZWpsT1VhUDRTU3FNckdSUVd1Zk5pSG11a1ZtWXo5bUFsN3pRQ2xyWTZEODNBOUwKQkVkMmNYa0NnWUVBMktRVmNhVDlGRFM2ZENPdHZFL2hPUDhSNVAvY21JSjFNMXhKY2hzL1IxcUdjd0RqWkZiRwp0RGFFNVNwVExidjg4d2hKb2FDbWhlbE9renFRaWIrNXNZU2tqZERVTThIUTlhWXk3aGlxT29jL3dFUlB6ZGRRCmUyelhYRHd2QWQ2VGc3OWpLV2NhOWZxcUw0azRWOVBtbHlrVjRUVDVoaHBhVkFqbi82VDZGYXNDZ1lFQTBWL1MKT0JSdzQraEIwU0JiUU1rNTcxdk1mUEdtR0dLb1BPRWRsRzBCY0RsZTF3Wk5yQ01mYVJWdUFMOFFuQ3ZOZXg2Two1UEtUN0tOeXh5Q21IOWlaQXZjeGRjbHF5aHNncHVtTDhhM0tsb1RuSU9vSHdobUVhMWxoVCtHY01QZzEvVTl4ClVqZEY2Y0tmSFFJd2FYYkNHUTVzaFhBUU0yVU5HbStYa0ZmeTQ4VUNnWUJ3d0cxOHVVOFNmaUx3b1VVaDlqMFYKQ2dRSk9IVmFWc09pMkl4TlBBc2lHdVpRNG94Mnc0Y2xjaDZXbXdHeGt0NmlxcFNQNzJuYjFrS1Q4KzRZRTFZVgpJeUQxd2xNL0lNZWRva050a2g2KzJYZC9uTTRnSnNqM2cvMU9QdkNFTzVCeENHSVd3VmZSNEFWRk9saTl0VWFWCk04ZjBienJTNWRKUFhGZEt3VlY3Z3dLQmdBak5NV1lnSGRyRzBiVjcyYm93ZTFuL2p1b1ZzbmpGOVBLU09BOGMKUWZvNHZ5N2syZkVKalBGNjhDUGg1RTNjWFlmMmNlVlgrVFh5YlFuSDZwUGVKQmlHMGJKMDVDTlkzcGVGcTlkZQpDZTBuNnh0c0d5VmlzemxjQ1lZMUlyN0FRR3pFb1N2bW5PN0Z1ckNhZmZTQkJJblBIR3JEbWpxKzNiMGx3Y1pVCm5DWk5Bb0dCQUpJYmZKSVNkbFlBalRwYnRVbVd0K2tlemNlWVdsUkxENHNOcWx2SWQ0U2JUbmEwR2FRSWRVSHIKWWU5RjhpTVgweWh1WWhzUXoraHlkZDhCVW44TXgwRVN6c2g3d0tpak5jenJpV3JUODlmYmlxVUpocFZvWUhWNwpQVExTZ1pSUDFyUU9UekpGVEkzdjMyNkg1dzFlZzJ0NUFOcnRWenFYTng4N2FOTXQwWGNQCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==",
			},
		},
		ReplicationFactor: 4,
		Master: ybv1alpha1.YBMasterSpec{
			Replicas:            3,
			MasterUIPort:        7001,
			MasterRPCPort:       7101,
			EnableLoadBalancer:  true,
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Storage: ybv1alpha1.YBStorageSpec{
				Count:        1,
				Size:         "1Gi",
				StorageClass: storageClassDefault,
			},
			Gflags: []ybv1alpha1.YBGFlagSpec{
				{
					Key:   "default_memory_limit_to_ram_ratio",
					Value: "0.85",
				},
			},
		},
		Tserver: ybv1alpha1.YBTServerSpec{
			Replicas:            5,
			TserverUIPort:       9001,
			TserverRPCPort:      9101,
			YCQLPort:            9042,
			YedisPort:           6379,
			YSQLPort:            5433,
			EnableLoadBalancer:  true,
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Storage: ybv1alpha1.YBStorageSpec{
				Count:        5,
				Size:         "100Gi",
				StorageClass: storageClassDefault,
			},
			Gflags: []ybv1alpha1.YBGFlagSpec{
				{
					Key:   "default_memory_limit_to_ram_ratio",
					Value: "0.85",
				},
			},
		},
	}
}
