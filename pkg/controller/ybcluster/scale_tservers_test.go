package ybcluster

import (
	"testing"

	"github.com/operator-framework/operator-sdk/pkg/status"
	"github.com/stretchr/testify/assert"
	yugabytev1alpha1 "github.com/yugabyte/yugabyte-k8s-operator/pkg/apis/yugabyte/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestAllowTServerStsUpdate(t *testing.T) {

	scaleDownCond := status.Condition{
		Type:   scalingDownTServersCondition,
		Status: corev1.ConditionTrue,
	}
	noScaleDownCond := status.Condition{
		Type:   scalingDownTServersCondition,
		Status: corev1.ConditionFalse,
	}

	dataMoveCond := status.Condition{
		Type:   movingDataCondition,
		Status: corev1.ConditionTrue,
	}

	noDataMoveCond := status.Condition{
		Type:   movingDataCondition,
		Status: corev1.ConditionFalse,
	}

	dataMoveBeforeScaleDownCluster := &yugabytev1alpha1.YBCluster{}
	dataMoveBeforeScaleDownCluster.Status.Conditions.SetCondition(dataMoveCond)
	dataMoveBeforeScaleDownCluster.Status.Conditions.SetCondition(scaleDownCond)

	dataMoveBeforeNoScaleDownCluster := &yugabytev1alpha1.YBCluster{}
	dataMoveBeforeNoScaleDownCluster.Status.Conditions.SetCondition(dataMoveCond)
	dataMoveBeforeNoScaleDownCluster.Status.Conditions.SetCondition(noScaleDownCond)

	noDataMoveBeforeScaleDownCluster := &yugabytev1alpha1.YBCluster{}
	noDataMoveBeforeScaleDownCluster.Status.Conditions.SetCondition(noDataMoveCond)
	noDataMoveBeforeScaleDownCluster.Status.Conditions.SetCondition(scaleDownCond)

	noDataMoveAfterScaleDownCluster := &yugabytev1alpha1.YBCluster{}
	noDataMoveAfterScaleDownCluster.Status.Conditions.SetCondition(scaleDownCond)
	noDataMoveAfterScaleDownCluster.Status.Conditions.SetCondition(noDataMoveCond)

	cases := []struct {
		name    string
		by      int32
		cluster *yugabytev1alpha1.YBCluster
		out     bool
		err     bool
	}{
		{"scale down no conditions set", int32(1), &yugabytev1alpha1.YBCluster{}, false, true},
		{"scale down DataMove ScaleDown", int32(2), dataMoveBeforeScaleDownCluster, false, false},
		{"scale down DataMove NoScaleDown", int32(2), dataMoveBeforeNoScaleDownCluster, false, false},
		{"scale down NoDataMove set before ScaleDown", int32(1), noDataMoveBeforeScaleDownCluster, false, false},
		{"scale down NoDataMove set after ScaleDown", int32(1), noDataMoveAfterScaleDownCluster, true, false},

		{"scale up no conditions set", int32(-1), &yugabytev1alpha1.YBCluster{}, true, false},
		{"scale up DataMove ScaleDown", int32(0), dataMoveBeforeScaleDownCluster, true, false},
		{"scale up DataMove NoScaleDown", int32(-2), dataMoveBeforeNoScaleDownCluster, true, false},
		{"scale up NoDataMove set before ScaleDown", int32(-1), noDataMoveBeforeScaleDownCluster, true, false},
		{"scale up NoDataMove set after ScaleDown", int32(-1), noDataMoveAfterScaleDownCluster, true, false},
	}

	for _, cas := range cases {
		t.Run(cas.name, func(t *testing.T) {
			got, err := allowTServerStsUpdate(cas.by, cas.cluster)
			assert.Equal(t, cas.out, got)
			if cas.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
