package ybcluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	ybv1alpha1 "github.com/yugabyte/yugabyte-k8s-operator/pkg/apis/yugabyte/v1alpha1"
)

func TestUpdateMasterHeadlessService(t *testing.T) {
	minimalCluster := &ybv1alpha1.YBCluster{
		Spec: *getMinimalClusterSpec(),
	}
	addDefaults(&minimalCluster.Spec)
	service, err := createMasterHeadlessService(minimalCluster)

	assert.Nil(t, err)
	assert.NotNil(t, service)

	newCluster := &ybv1alpha1.YBCluster{
		Spec: *getMinimalClusterSpec(),
	}

	newCluster.Spec.Master.MasterUIPort = 1234
	newCluster.Spec.Master.MasterRPCPort = 2345
	addDefaults(&newCluster.Spec)

	updateMasterHeadlessService(newCluster, service)

	assert.Equal(t, int32(1234), service.Spec.Ports[0].Port)
	assert.Equal(t, int32(2345), service.Spec.Ports[1].Port)
}

func TestUpdateTServerHeadlessService(t *testing.T) {
	minimalCluster := &ybv1alpha1.YBCluster{
		Spec: *getMinimalClusterSpec(),
	}
	addDefaults(&minimalCluster.Spec)
	service, err := createTServerHeadlessService(minimalCluster)

	assert.Nil(t, err)
	assert.NotNil(t, service)

	newCluster := &ybv1alpha1.YBCluster{
		Spec: *getMinimalClusterSpec(),
	}

	newCluster.Spec.Tserver.TserverUIPort = 1234
	newCluster.Spec.Tserver.TserverRPCPort = 2345
	newCluster.Spec.Tserver.YCQLPort = 3456
	newCluster.Spec.Tserver.YedisPort = 4567
	newCluster.Spec.Tserver.YSQLPort = 5678
	addDefaults(&newCluster.Spec)

	updateTServerHeadlessService(newCluster, service)

	assert.Equal(t, int32(2345), service.Spec.Ports[0].Port)
	assert.Equal(t, int32(3456), service.Spec.Ports[1].Port)
	assert.Equal(t, int32(4567), service.Spec.Ports[2].Port)
	assert.Equal(t, int32(5678), service.Spec.Ports[3].Port)
	assert.Equal(t, int32(1234), service.Spec.Ports[4].Port)
}

func TestUpdateMasterUIService(t *testing.T) {
	minimalCluster := &ybv1alpha1.YBCluster{
		Spec: *getMinimalClusterSpec(),
	}
	addDefaults(&minimalCluster.Spec)
	service, err := createMasterUIService(minimalCluster)

	assert.Nil(t, err)
	assert.NotNil(t, service)

	newCluster := &ybv1alpha1.YBCluster{
		Spec: *getMinimalClusterSpec(),
	}

	newCluster.Spec.Master.MasterUIPort = 1234
	addDefaults(&newCluster.Spec)

	updateMasterHeadlessService(newCluster, service)

	assert.Equal(t, int32(1234), service.Spec.Ports[0].Port)
}

func TestUpdateTServerUIService(t *testing.T) {
	minimalCluster := &ybv1alpha1.YBCluster{
		Spec: *getMinimalClusterSpec(),
	}
	minimalCluster.Spec.Tserver.TserverUIPort = 9876
	addDefaults(&minimalCluster.Spec)
	service, err := createTServerUIService(minimalCluster)

	assert.Nil(t, err)
	assert.NotNil(t, service)

	newCluster := &ybv1alpha1.YBCluster{
		Spec: *getMinimalClusterSpec(),
	}

	newCluster.Spec.Tserver.TserverUIPort = 1234
	addDefaults(&newCluster.Spec)

	updateTServerUIService(newCluster, service)

	assert.Equal(t, int32(1234), service.Spec.Ports[0].Port)
}

func TestUpdateMasterStatefulset(t *testing.T) {
	minimalCluster := &ybv1alpha1.YBCluster{
		Spec: *getMinimalClusterSpec(),
	}
	addDefaults(&minimalCluster.Spec)
	sfs, err := createMasterStatefulset(minimalCluster)

	assert.Nil(t, err)
	assert.NotNil(t, sfs)

	newCluster := &ybv1alpha1.YBCluster{
		Spec: *getMinimalClusterSpec(),
	}

	newCluster.Spec.Master.MasterUIPort = 1234
	newCluster.Spec.Master.MasterRPCPort = 2345
	newCluster.Spec.Master.Replicas = 4
	newCluster.Spec.Master.Storage.Count = 3
	newCluster.Spec.Master.Gflags = []ybv1alpha1.YBGFlagSpec{
		{
			Key:   "key1",
			Value: "value1",
		},
	}

	addDefaults(&newCluster.Spec)

	updateMasterStatefulset(newCluster, sfs)

	assert.Equal(t, int32(4), *sfs.Spec.Replicas)
	assert.Equal(t, 11, len(sfs.Spec.Template.Spec.Containers[0].Command))
	assert.Equal(t, int32(1234), sfs.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
	assert.Equal(t, 3, len(sfs.Spec.Template.Spec.Containers[0].VolumeMounts))
	assert.Equal(t, 3, len(sfs.Spec.VolumeClaimTemplates))
}

func TestUpdateTServerStatefulset(t *testing.T) {
	minimalCluster := &ybv1alpha1.YBCluster{
		Spec: *getMinimalClusterSpec(),
	}
	addDefaults(&minimalCluster.Spec)
	sfs, err := createTServerStatefulset(minimalCluster)

	assert.Nil(t, err)
	assert.NotNil(t, sfs)

	newCluster := &ybv1alpha1.YBCluster{
		Spec: *getMinimalClusterSpec(),
	}

	newCluster.Spec.Tserver.TserverUIPort = 1234
	newCluster.Spec.Tserver.TserverRPCPort = 2345
	newCluster.Spec.Tserver.YCQLPort = 3456
	newCluster.Spec.Tserver.YedisPort = 4567
	newCluster.Spec.Tserver.YSQLPort = 5678
	newCluster.Spec.Tserver.Replicas = 5
	newCluster.Spec.Tserver.Storage.Count = 4
	newCluster.Spec.Tserver.Gflags = []ybv1alpha1.YBGFlagSpec{
		{
			Key:   "key1",
			Value: "value1",
		},
		{
			Key:   "key2",
			Value: "value2",
		},
	}

	addDefaults(&newCluster.Spec)

	updateTServerStatefulset(newCluster, sfs)

	assert.Equal(t, int32(5), *sfs.Spec.Replicas)
	assert.Equal(t, 12, len(sfs.Spec.Template.Spec.Containers[0].Command))
	assert.Equal(t, int32(1234), sfs.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
	assert.Equal(t, int32(2345), sfs.Spec.Template.Spec.Containers[0].Ports[1].ContainerPort)
	assert.Equal(t, int32(3456), sfs.Spec.Template.Spec.Containers[0].Ports[2].ContainerPort)
	assert.Equal(t, int32(4567), sfs.Spec.Template.Spec.Containers[0].Ports[3].ContainerPort)
	assert.Equal(t, int32(5678), sfs.Spec.Template.Spec.Containers[0].Ports[4].ContainerPort)
	assert.Equal(t, 4, len(sfs.Spec.Template.Spec.Containers[0].VolumeMounts))
	assert.Equal(t, 4, len(sfs.Spec.VolumeClaimTemplates))
}
