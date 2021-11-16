package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	yugabytecomv1alpha1 "github.com/yugabyte/yugabyte-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestAllowTServerStsUpdate(t *testing.T) {

	scaleDownCond := yugabytecomv1alpha1.YBClusterCondition{
		Type:   scalingDownTServersCondition,
		Status: corev1.ConditionTrue,
	}
	noScaleDownCond := yugabytecomv1alpha1.YBClusterCondition{
		Type:   scalingDownTServersCondition,
		Status: corev1.ConditionFalse,
	}

	dataMoveCond := yugabytecomv1alpha1.YBClusterCondition{
		Type:   movingDataCondition,
		Status: corev1.ConditionTrue,
	}

	noDataMoveCond := yugabytecomv1alpha1.YBClusterCondition{
		Type:   movingDataCondition,
		Status: corev1.ConditionFalse,
	}

	dataMoveBeforeScaleDownCluster := &yugabytecomv1alpha1.YBCluster{}
	dataMoveBeforeScaleDownCluster.Status.Conditions = append(dataMoveBeforeScaleDownCluster.Status.Conditions, dataMoveCond)
	dataMoveBeforeScaleDownCluster.Status.Conditions = append(dataMoveBeforeScaleDownCluster.Status.Conditions, scaleDownCond)

	dataMoveBeforeNoScaleDownCluster := &yugabytecomv1alpha1.YBCluster{}
	dataMoveBeforeNoScaleDownCluster.Status.Conditions = append(dataMoveBeforeNoScaleDownCluster.Status.Conditions, dataMoveCond)
	dataMoveBeforeNoScaleDownCluster.Status.Conditions = append(dataMoveBeforeNoScaleDownCluster.Status.Conditions, noScaleDownCond)

	noDataMoveBeforeScaleDownCluster := &yugabytecomv1alpha1.YBCluster{}
	noDataMoveBeforeScaleDownCluster.Status.Conditions = append(noDataMoveBeforeScaleDownCluster.Status.Conditions, noDataMoveCond)
	noDataMoveBeforeScaleDownCluster.Status.Conditions = append(noDataMoveBeforeScaleDownCluster.Status.Conditions, scaleDownCond)

	noDataMoveAfterScaleDownCluster := &yugabytecomv1alpha1.YBCluster{}
	noDataMoveAfterScaleDownCluster.Status.Conditions = append(noDataMoveAfterScaleDownCluster.Status.Conditions, scaleDownCond)
	noDataMoveAfterScaleDownCluster.Status.Conditions = append(noDataMoveAfterScaleDownCluster.Status.Conditions, noDataMoveCond)

	cases := []struct {
		name    string
		by      int32
		cluster *yugabytecomv1alpha1.YBCluster
		out     bool
		err     bool
	}{
		{"scale down no conditions set", int32(1), &yugabytecomv1alpha1.YBCluster{}, false, true},
		{"scale down DataMove ScaleDown", int32(2), dataMoveBeforeScaleDownCluster, false, false},
		{"scale down DataMove NoScaleDown", int32(2), dataMoveBeforeNoScaleDownCluster, false, false},
		{"scale down NoDataMove set before ScaleDown", int32(1), noDataMoveBeforeScaleDownCluster, false, false},
		{"scale down NoDataMove set after ScaleDown", int32(1), noDataMoveAfterScaleDownCluster, true, false},

		{"scale up no conditions set", int32(-1), &yugabytecomv1alpha1.YBCluster{}, true, false},
		{"scale up DataMove ScaleDown", int32(0), dataMoveBeforeScaleDownCluster, true, false},
		{"scale up DataMove NoScaleDown", int32(-2), dataMoveBeforeNoScaleDownCluster, true, false},
		{"scale up NoDataMove set before ScaleDown", int32(-1), noDataMoveBeforeScaleDownCluster, true, false},
		{"scale up NoDataMove set after ScaleDown", int32(-1), noDataMoveAfterScaleDownCluster, true, false},
	}

	for _, cas := range cases {
		t.Run(cas.name, func(t *testing.T) {
			got, err := allowTServerStsUpdate(cas.by, cas.cluster, log.FromContext(context.TODO()))
			assert.Equal(t, cas.out, got)
			if cas.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
