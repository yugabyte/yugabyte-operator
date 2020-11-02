package ybcluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/operator-framework/operator-sdk/pkg/status"
	yugabytev1alpha1 "github.com/yugabyte/yugabyte-k8s-operator/pkg/apis/yugabyte/v1alpha1"
	"github.com/yugabyte/yugabyte-k8s-operator/pkg/kube"
	"github.com/yugabyte/yugabyte-k8s-operator/pkg/ybconfig"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	blacklistAnnotation = "yugabyte.com/blacklist"

	ybAdminBinary                   = "/home/yugabyte/bin/yb-admin"
	ybAdminGetUniverseConfigCmd     = "get_universe_config"
	ybAdminChangeBlacklistCmd       = "change_blacklist"
	ybAdminGetLoadMoveCompletionCmd = "get_load_move_completion"
	ybAdminMasterAddressesFlag      = "--master_addresses"
)

type ybAdminBlacklistOperation string

const (
	ybAdminBlacklistAddOp    ybAdminBlacklistOperation = "ADD"
	ybAdminBlacklistRemoveOp ybAdminBlacklistOperation = "REMOVE"
)

const (
	movingDataCondition          status.ConditionType   = "MovingData"
	dataMoveInProgress           status.ConditionReason = "DataMoveInProgress"
	dataMoveInProgressMsg        string                 = "data move operation is in progress"
	noDataMoveInProgress         status.ConditionReason = "NoDataMoveInProgress"
	noDataMoveInProgressMsg      string                 = "no data move operation is in progress"
	scalingDownTServersCondition status.ConditionType   = "ScalingDownTServers"
	scaleDownInProgress          status.ConditionReason = "ScaleDownInProgress"
	scaleDownInProgressMsg       string                 = "one or more TServer(s) are scaling down"
	noScaleDownInProgress        status.ConditionReason = "NoScaleDownInProgress"
	noScaleDownInProgressMsg     string                 = "no TServer(s) are scaling down"
)

const (
	scalingDownTServersEventReason string = "ScalingDownTServers"
	scaledDownTServersEventReason  string = "ScaledDownTServers"
)

// scaleTServers determines if TServers are going to be scaled up or
// scaled down. If scale down operation is required, it blacklists
// TServer pods. Retruns boolean indicating if StatefulSet should be
// updated or not.
func (r *ReconcileYBCluster) scaleTServers(currentReplicas int32, cluster *yugabytev1alpha1.YBCluster) (bool, error) {
	// Ignore new/changed replica count if scale down
	// operation is in progress
	tserverScaleDownCond := cluster.Status.Conditions.GetCondition(scalingDownTServersCondition)
	if tserverScaleDownCond == nil || tserverScaleDownCond.IsFalse() {
		cluster.Status.TargetedTServerReplicas = cluster.Spec.Tserver.Replicas
		if err := r.client.Status().Update(context.TODO(), cluster); err != nil {
			return false, err
		}
	}
	scaleDownBy := currentReplicas - cluster.Status.TargetedTServerReplicas

	if scaleDownBy > 0 {
		logger.Infof("scaling down TServer replicas by %d.", scaleDownBy)

		tserverScaleCond := status.Condition{
			Type:    scalingDownTServersCondition,
			Status:  corev1.ConditionTrue,
			Reason:  scaleDownInProgress,
			Message: scaleDownInProgressMsg,
		}
		logger.Infof("updating Status condition %s: %s", tserverScaleCond.Type, tserverScaleCond.Status)
		cluster.Status.Conditions.SetCondition(tserverScaleCond)
		if err := r.client.Status().Update(context.TODO(), cluster); err != nil {
			return false, err
		}

		if err := r.blacklistPods(cluster, scaleDownBy); err != nil {
			return false, err
		}
	}

	return allowTServerStsUpdate(scaleDownBy, cluster)
}

// blacklistPods adds yugabyte.com/blacklist: true annotation to the
// TServer pods
func (r *ReconcileYBCluster) blacklistPods(cluster *yugabytev1alpha1.YBCluster, cnt int32) error {
	logger.Infof("adding blacklist annotation to %d TServer pods", cnt)
	scalingDownTo := cluster.Status.TargetedTServerReplicas
	tserverReplicas := scalingDownTo + cnt
	for podNum := tserverReplicas - 1; podNum >= scalingDownTo; podNum-- {
		pod := &corev1.Pod{}
		err := r.client.Get(context.TODO(), types.NamespacedName{
			Namespace: cluster.GetNamespace(),
			Name:      fmt.Sprintf("%s-%d", tserverName, podNum),
		}, pod)
		if err != nil {
			return err
		}

		if pod.Annotations == nil {
			pod.SetAnnotations(map[string]string{blacklistAnnotation: "true"})
		} else {
			pod.Annotations[blacklistAnnotation] = "true"
		}

		if err = r.client.Update(context.TODO(), pod); err != nil {
			return err
		}
	}
	return nil
}

// allowTServerStsUpdate decides if TServer StatefulSet should be
// updated or not. Update is allowed for scale up directly. If it's a
// scale down, then it checks if data move operation has been
// completed.
func allowTServerStsUpdate(scaleDownBy int32, cluster *yugabytev1alpha1.YBCluster) (bool, error) {
	// Allow scale up directly
	if scaleDownBy <= 0 {
		return true, nil
	}

	dataMoveCond := cluster.Status.Conditions.GetCondition(movingDataCondition)
	tserverScaleDownCond := cluster.Status.Conditions.GetCondition(scalingDownTServersCondition)
	if dataMoveCond == nil || tserverScaleDownCond == nil {
		err := fmt.Errorf("status condition %s or %s is nil", movingDataCondition, scalingDownTServersCondition)
		logger.Error(err)
		return false, err
	}

	// Allow scale down if data move operation has been completed
	if dataMoveCond.IsFalse() {
		// TODO(bhavin192): add heartbeat time so that we can
		// handle the 0 tablets on a TServer case. Should have
		// gap of at least 5 minutes.
		if dataMoveCond.LastTransitionTime.After(tserverScaleDownCond.LastTransitionTime.Time) {
			return true, nil
		}
	}
	return false, nil
}

// syncBlacklist makes sure that the pods with blacklist annotation
// are added to the blacklist in YB-Master configuration. If the
// annotation is missing, then the pod is removed from YB-Master's
// blacklist.
func (r *ReconcileYBCluster) syncBlacklist(cluster *yugabytev1alpha1.YBCluster) error {
	// Get list of all the YB-TServer pods
	pods := &corev1.PodList{}

	labels := createAppLabels(tserverName)
	labels[ybClusterNameLabel] = cluster.GetName()
	opts := []client.ListOption{
		client.InNamespace(cluster.GetNamespace()),
		client.MatchingLabels(labels),
	}

	err := r.client.List(context.TODO(), pods, opts...)
	if err != nil {
		return err
	}

	// Fetch current blacklist from YB-Master
	masterPod := fmt.Sprintf("%s-%d", masterName, 0)
	getConfigCmd := runWithShell("bash",
		[]string{
			ybAdminBinary,
			ybAdminMasterAddressesFlag,
			getMasterAddresses(
				cluster.Namespace,
				cluster.Spec.Master.MasterRPCPort,
				cluster.Spec.Master.Replicas,
			),
			ybAdminGetUniverseConfigCmd,
		})

	logger.Infof("running command 'yb-admin %s' in YB-Master pod: %s, command: %q", ybAdminGetUniverseConfigCmd, masterPod, getConfigCmd)
	cout, _, err := kube.Exec(r.config, cluster.Namespace, masterPod, "", getConfigCmd, nil)
	if err != nil {
		return err
	}
	// TODO(bhavin192): improve this log line
	// logger.Infof("got the config, cout: %s, cerr: %s", cout, cerr)

	universeCfg, err := ybconfig.NewFromJSON([]byte(cout))
	if err != nil {
		return err
	}

	currentBl := universeCfg.GetBlacklist()
	logger.Infof("current blacklist from YB-Master: %q", currentBl)

	for _, pod := range pods.Items {
		podHostPort := fmt.Sprintf(
			"%s.%s.%s.svc.cluster.local:%d",
			pod.ObjectMeta.Name,
			tserverNamePlural,
			cluster.Namespace,
			cluster.Spec.Tserver.TserverRPCPort,
		)

		operation := getBlacklistOperation(pod)
		if containsString(currentBl, podHostPort) {
			if operation == ybAdminBlacklistAddOp {
				logger.Infof("pod %s is already in YB-Master blacklist, skipping.", podHostPort)
				continue
			}
		} else {
			if operation == ybAdminBlacklistRemoveOp {
				logger.Infof("pod %s is not in YB-Master blacklist, skipping.", podHostPort)
				continue
			}
		}

		modBlacklistCmd := runWithShell("bash",
			[]string{
				ybAdminBinary,
				ybAdminMasterAddressesFlag,
				getMasterAddresses(
					cluster.Namespace,
					cluster.Spec.Master.MasterRPCPort,
					cluster.Spec.Master.Replicas,
				),
				ybAdminChangeBlacklistCmd,
				string(operation),
				podHostPort,
			})

		// blacklist it or remove it
		logger.Infof("running command 'yb-admin %s' in YB-Master pod: %s, command: %q", ybAdminChangeBlacklistCmd, masterPod, modBlacklistCmd)
		_, _, err := kube.Exec(r.config, cluster.Namespace, masterPod, "", modBlacklistCmd, nil)
		if err != nil {
			return err
		}

		logger.Infof("modified the blacklist, pod: %s, operation: %s", pod.ObjectMeta.Name, operation)

		// TODO(bhavin192): improve this log line
		// logger.Infof("%s %s to/from blacklist out: %s, err: %s", pod.ObjectMeta.Name, operation, cout, cerr)

		// TODO(bhavin192): if there is no error, should we
		// just assume that the pod has been added to the
		// blacklist and don't query the blacklist to verify
		// that?

		// TODO(bhavin192): should update the whole PodList at once?
		// TODO(bhavin192): mark the pod as synced?
	}
	return nil
}

// getBlacklistOperation returns the blacklist operation to be
// performed on given pod. Returns ybAdminBlacklistRemoveOp if the
// blacklistAnnotation is "false" or if it doesn't exist. Returns
// ybAdminBlacklistAddOp otherwise.
func getBlacklistOperation(p corev1.Pod) ybAdminBlacklistOperation {
	if p.Annotations == nil {
		return ybAdminBlacklistRemoveOp
	}
	if _, ok := p.Annotations[blacklistAnnotation]; !ok {
		return ybAdminBlacklistRemoveOp
	}
	if p.Annotations[blacklistAnnotation] == "false" {
		return ybAdminBlacklistRemoveOp
	}
	return ybAdminBlacklistAddOp
}

// checkDataMoveProgress queries YB-Master for the progress of data
// move operation. Sets the value of status condition
// movingDataCondition accordingly.
func (r *ReconcileYBCluster) checkDataMoveProgress(cluster *yugabytev1alpha1.YBCluster) error {
	cmd := runWithShell("bash",
		[]string{
			ybAdminBinary,
			ybAdminMasterAddressesFlag,
			getMasterAddresses(
				cluster.Namespace,
				cluster.Spec.Master.MasterRPCPort,
				cluster.Spec.Master.Replicas,
			),
			ybAdminGetLoadMoveCompletionCmd,
		},
	)
	masterPod := fmt.Sprintf("%s-%d", masterName, 0)
	logger.Infof("running command 'yb-admin %s' in YB-Master pod: %s, command: %q", ybAdminGetLoadMoveCompletionCmd, masterPod, cmd)
	cout, _, err := kube.Exec(r.config, cluster.Namespace, masterPod, "", cmd, nil)
	if err != nil {
		return err
	}

	// TODO(bhavin192): improve this log line
	// logger.Infof("get_load_move_completion: out: %s, err: %s", cout, cerr)
	p := cout[strings.Index(cout, "= ")+2 : strings.Index(cout, " :")]
	logger.Infof("current data move progress: %s", p)

	// Toggle the MovingData condition
	cond := status.Condition{Type: movingDataCondition}
	if p != "100" {
		cond.Status = corev1.ConditionTrue
		cond.Reason = dataMoveInProgress
		cond.Message = dataMoveInProgressMsg
	} else {
		cond.Status = corev1.ConditionFalse
		cond.Reason = noDataMoveInProgress
		cond.Message = noDataMoveInProgressMsg
	}

	logger.Infof("updating Status condition %s: %s", cond.Type, cond.Status)
	cluster.Status.Conditions.SetCondition(cond)
	if err := r.client.Status().Update(context.TODO(), cluster); err != nil {
		return err
	}
	return nil
}
