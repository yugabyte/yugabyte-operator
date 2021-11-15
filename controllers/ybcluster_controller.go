/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	yugabytecomv1alpha1 "github.com/yugabyte/yugabyte-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	customResourceName          = "ybcluster"
	customResourceNamePlural    = "ybclusters"
	masterName                  = "yb-master"
	masterNamePlural            = "yb-masters"
	tserverName                 = "yb-tserver"
	tserverNamePlural           = "yb-tservers"
	masterSecretName            = "yb-master-yugabyte-tls-cert"
	tserverSecretName           = "yb-tserver-yugabyte-tls-cert"
	masterUIServiceName         = "yb-master-ui"
	tserverUIServiceName        = "yb-tserver-ui"
	masterUIPortDefault         = int32(7000)
	masterRPCPortDefault        = int32(7100)
	tserverUIPortDefault        = int32(9000)
	tserverRPCPortDefault       = int32(9100)
	ycqlPortDefault             = int32(9042)
	ycqlPortName                = "ycql"
	yedisPortDefault            = int32(6379)
	yedisPortName               = "yedis"
	ysqlPortDefault             = int32(5433)
	ysqlPortName                = "ysql"
	masterContainerUIPortName   = "master-ui"
	masterContainerRPCPortName  = "master-rpc"
	tserverContainerUIPortName  = "tserver-ui"
	tserverContainerRPCPortName = "tserver-rpc"
	uiPortName                  = "ui"
	rpcPortName                 = "rpc-port"
	volumeMountName             = "datadir"
	volumeMountPath             = "/mnt/data"
	secretMountPath             = "/opt/certs/yugabyte"
	envGetHostsFrom             = "GET_HOSTS_FROM"
	envGetHostsFromVal          = "dns"
	envPodIP                    = "POD_IP"
	envPodIPVal                 = "status.podIP"
	envPodName                  = "POD_NAME"
	envPodNameVal               = "metadata.name"
	yugabyteDBImageName         = "yugabytedb/yugabyte:2.9.1.0-b140"
	imageRepositoryDefault      = "yugabytedb/yugabyte"
	imageTagDefault             = "2.9.1.0-b140"
	imagePullPolicyDefault      = corev1.PullIfNotPresent
	podManagementPolicyDefault  = appsv1.ParallelPodManagement
	storageCountDefault         = int32(1)
	storageClassDefault         = "standard"
	domainDefault               = "cluster.local"
	labelHostname               = "kubernetes.io/hostname"
	appLabel                    = "app"
)

// YBClusterReconciler reconciles a YBCluster object
type YBClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	config   *rest.Config
	recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=yugabyte.com,resources=ybclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=yugabyte.com,resources=ybclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=yugabyte.com,resources=ybclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the YBCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *YBClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciling YBCluster")

	// Fetch the YBCluster instance
	cluster := &yugabytecomv1alpha1.YBCluster{}
	err := r.Get(context.TODO(), req.NamespacedName, cluster)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	validateCR(&cluster.Spec)
	addDefaults(&cluster.Spec)

	if err = r.reconcileSecrets(cluster, logger); err != nil {
		// Error reconciling secrets - requeue the request.
		return reconcile.Result{}, err
	}

	if err = r.reconcileHeadlessServices(cluster, logger); err != nil {
		// Error reconciling headless services - requeue the request.
		return reconcile.Result{}, err
	}

	if err = r.reconcileUIServices(cluster, logger); err != nil {
		// Error reconciling ui services - requeue the request.
		return reconcile.Result{}, err
	}

	result, err := r.reconcileStatefulsets(cluster, logger)
	if err != nil {
		return result, err
	}

	if err := r.syncBlacklist(cluster, logger); err != nil {
		// Error syncing the blacklist - requeue the request.
		return reconcile.Result{}, err
	}

	if err := r.checkDataMoveProgress(cluster, logger); err != nil {
		// Error checking data move progress - requeue the request.
		return reconcile.Result{}, err
	}

	return result, nil
}

func (r *YBClusterReconciler) reconcileSecrets(cluster *yugabytecomv1alpha1.YBCluster, logger logr.Logger) error {

	masterSecret, err := r.fetchSecret(cluster.Namespace, false)
	if err != nil {
		return err
	}

	tserverSecret, err := r.fetchSecret(cluster.Namespace, true)
	if err != nil {
		return err
	}

	if !cluster.Spec.TLS.Enabled {
		// delete if object exists.
		if masterSecret != nil {
			logger.Info("deleting master secret")
			if err := r.Delete(context.TODO(), masterSecret); err != nil {
				logger.Error(err, "failed to delete existing master secrets object.")
				return err
			}
		}

		if tserverSecret != nil {
			logger.Info("deleting tserver secret")
			if err := r.Delete(context.TODO(), tserverSecret); err != nil {
				logger.Error(err, "failed to delete existing tserver secrets object.")
				return err
			}
		}
	} else {
		// check if object exists.Update it, if yes. Create it, if it doesn't
		if masterSecret != nil {
			// Updating
			logger.Info("updating master secret")
			if err := updateMasterSecret(cluster, masterSecret); err != nil {
				logger.Error(err, "failed to update master secrets object.")
				return err
			}

			if err := r.Update(context.TODO(), masterSecret); err != nil {
				logger.Error(err, "failed to update master secrets object.")
				return err
			}
		} else {
			// Creating
			masterSecret, err := createMasterSecret(cluster)
			if err != nil {
				// Error creating master secret object
				logger.Error(err, "forming master secret object failed.")
				return err
			}

			logger.Info("creating a new Secret %s for YBMasters in namespace %s", masterSecret.Name, masterSecret.Namespace)
			// Set YBCluster instance as the owner and controller for master secret
			if err := controllerutil.SetControllerReference(cluster, masterSecret, r.Scheme); err != nil {
				return err
			}

			err = r.Create(context.TODO(), masterSecret)
			if err != nil {
				return err
			}
		}

		if tserverSecret != nil {
			// Updating
			logger.Info("updating tserver secret")
			if err := updateTServerSecret(cluster, tserverSecret); err != nil {
				logger.Error(err, "failed to update tserver secrets object.")
				return err
			}

			if err := r.Update(context.TODO(), tserverSecret); err != nil {
				logger.Error(err, "failed to update tserver secrets object.")
				return err
			}
		} else {
			// Creating
			tserverSecret, err := createTServerSecret(cluster)
			if err != nil {
				// Error creating master secret object
				logger.Error(err, "forming master secret object failed.")
				return err
			}

			logger.Info("creating a new Secret %s for YBTServers in namespace %s", tserverSecret.Name, tserverSecret.Namespace)
			// Set YBCluster instance as the owner and controller for tserver secret
			if err := controllerutil.SetControllerReference(cluster, tserverSecret, r.Scheme); err != nil {
				return err
			}

			err = r.Create(context.TODO(), tserverSecret)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *YBClusterReconciler) fetchSecret(namespace string, isTserver bool) (*corev1.Secret, error) {
	name := masterSecretName

	if isTserver {
		name = tserverSecretName
	}

	found := &corev1.Secret{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return found, nil
}

func (r *YBClusterReconciler) reconcileHeadlessServices(cluster *yugabytecomv1alpha1.YBCluster, logger logr.Logger) error {
	// Check if master headless service already exists
	found := &corev1.Service{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: masterNamePlural, Namespace: cluster.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		masterHeadlessService, err := createMasterHeadlessService(cluster)
		if err != nil {
			// Error creating master headless service object
			logger.Error(err, "forming master headless service object failed.")
			return err
		}

		// Set YBCluster instance as the owner and controller for master headless service
		if err := controllerutil.SetControllerReference(cluster, masterHeadlessService, r.Scheme); err != nil {
			return err
		}
		logger.Info("creating a new Headless Service %s for YBMasters in namespace %s", masterHeadlessService.Name, masterHeadlessService.Namespace)
		err = r.Create(context.TODO(), masterHeadlessService)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		logger.Info("updating master headless service")
		updateMasterHeadlessService(cluster, found)
		if err := r.Update(context.TODO(), found); err != nil {
			logger.Error(err, "failed to update master headless service object.")
			return err
		}
	}

	// Check if tserver headless service already exists
	found = &corev1.Service{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: tserverNamePlural, Namespace: cluster.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		tserverHeadlessService, err := createTServerHeadlessService(cluster)
		if err != nil {
			// Error creating tserver headless service object
			logger.Error(err, "forming tserver headless service object failed.")
			return err
		}

		// Set YBCluster instance as the owner and controller for tserver headless service
		if err := controllerutil.SetControllerReference(cluster, tserverHeadlessService, r.Scheme); err != nil {
			return err
		}

		logger.Info("creating a new Headless Service %s for YBTServers in namespace %s", tserverHeadlessService.Name, tserverHeadlessService.Namespace)

		err = r.Create(context.TODO(), tserverHeadlessService)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		logger.Info("updating tserver headless service")
		updateTServerHeadlessService(cluster, found)
		if err := r.Update(context.TODO(), found); err != nil {
			logger.Error(err, "failed to update tserver headless service object.")
			return err
		}
	}

	return nil
}

func (r *YBClusterReconciler) reconcileUIServices(cluster *yugabytecomv1alpha1.YBCluster, logger logr.Logger) error {
	// Check if master ui service already exists
	found := &corev1.Service{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: masterUIServiceName, Namespace: cluster.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		masterUIService, err := createMasterUIService(cluster)
		if err != nil {
			// Error creating master ui service object
			logger.Error(err, "forming master ui service object failed.")
			return err
		}

		// Set YBCluster instance as the owner and controller for master ui service
		if err := controllerutil.SetControllerReference(cluster, masterUIService, r.Scheme); err != nil {
			return err
		}

		logger.Info("creating a new UI Service %s for YBMasters in namespace %s", masterUIService.Name, masterUIService.Namespace)
		err = r.Create(context.TODO(), masterUIService)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		logger.Info("updating master ui service")
		updateMasterUIService(cluster, found)
		if err := r.Update(context.TODO(), found); err != nil {
			logger.Error(err, "failed to update master ui service object.")
			return err
		}
	}

	// Check if tserver ui service already exists
	found = &corev1.Service{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: tserverUIServiceName, Namespace: cluster.Namespace}, found)
	if err != nil && errors.IsNotFound(err) && cluster.Spec.Tserver.TserverUIPort > 0 {
		// Create TServer ui service, if it didn't existed.
		tserverUIService, err := createTServerUIService(cluster)
		if err != nil {
			// Error creating tserver ui service object
			logger.Error(err, "forming tserver UI Service object failed.")
			return err
		}

		// Set YBCluster instance as the owner and controller for tserver ui service
		if err := controllerutil.SetControllerReference(cluster, tserverUIService, r.Scheme); err != nil {
			return err
		}

		logger.Info("creating a new ui service %s for YBTServers in namespace %s", tserverUIService.Name, tserverUIService.Namespace)

		err = r.Create(context.TODO(), tserverUIService)
		if err != nil {
			return err
		}
	} else if err == nil && cluster.Spec.Tserver.TserverUIPort <= 0 {
		// Delete the service if it existed before & it is not needed going forward.
		logger.Info("deleting tserver ui service")
		if err := r.Delete(context.TODO(), found); err != nil {
			logger.Error(err, "failed to delete tserver ui service object.")
			return err
		}
	} else if err == nil && cluster.Spec.Tserver.TserverUIPort > 0 {
		// Update the service if it existed before & is needed in the new spec.
		logger.Info("updating tserver ui service")
		updateTServerUIService(cluster, found)
		if err := r.Update(context.TODO(), found); err != nil {
			logger.Error(err, "failed to update tserver ui service object.")
			return err
		}
	} else if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func (r *YBClusterReconciler) reconcileStatefulsets(cluster *yugabytecomv1alpha1.YBCluster, logger logr.Logger) (reconcile.Result, error) {
	result := reconcile.Result{}
	// Check if master statefulset already exists
	found := &appsv1.StatefulSet{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: masterName, Namespace: cluster.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		masterStatefulset, err := createMasterStatefulset(cluster)
		if err != nil {
			// Error creating master statefulset object
			logger.Error(err, "forming master statefulset object failed.")
			return result, err
		}

		// Set YBCluster instance as the owner and controller for master statefulset
		if err := controllerutil.SetControllerReference(cluster, masterStatefulset, r.Scheme); err != nil {
			return result, err
		}
		logger.Info("creating a new Statefulset %s for YBMasters in namespace %s", masterStatefulset.Name, masterStatefulset.Namespace)
		err = r.Create(context.TODO(), masterStatefulset)
		if err != nil {
			return result, err
		}
	} else if err != nil {
		return result, err
	} else {
		logger.Info("updating master statefulset")
		updateMasterStatefulset(cluster, found)
		if err := r.Update(context.TODO(), found); err != nil {
			logger.Error(err, "failed to update master statefulset object.")
			return result, err
		}
	}

	tserverStatefulset, err := createTServerStatefulset(cluster)
	if err != nil {
		// Error creating tserver statefulset object
		logger.Error(err, "forming tserver statefulset object failed.")
		return result, err
	}

	// Set YBCluster instance as the owner and controller for tserver statefulset
	if err := controllerutil.SetControllerReference(cluster, tserverStatefulset, r.Scheme); err != nil {
		return result, err
	}

	// Check if tserver statefulset already exists
	found = &appsv1.StatefulSet{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: tserverStatefulset.Name, Namespace: tserverStatefulset.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("creating a new Statefulset %s for YBTServers in namespace %s", tserverStatefulset.Name, tserverStatefulset.Namespace)

		err = r.Create(context.TODO(), tserverStatefulset)
		if err != nil {
			return result, err
		}
	} else if err != nil {
		return result, err
	} else {
		// Don't requeue if TServer replica count is less than
		// cluster replication factor
		if cluster.Spec.Tserver.Replicas < cluster.Spec.ReplicationFactor {
			logger.Error(nil, "TServer replica count cannot be less than the replication factor of the cluster: '%d' < '%d'.", cluster.Spec.Tserver.Replicas, cluster.Spec.ReplicationFactor)
			return result, nil
		}

		allowStsUpdate, err := r.scaleTServers(*found.Spec.Replicas, cluster, logger)
		if err != nil {
			return result, err
		}

		if allowStsUpdate {
			logger.Info("updating tserver statefulset")
			updateTServerStatefulset(cluster, found)
			if err := r.Update(context.TODO(), found); err != nil {
				logger.Error(err, "failed to update tserver statefulset object.")
				return result, err
			}

			for _, condition := range cluster.Status.Conditions {
				if condition.Type == scalingDownTServersCondition {
					msg := "Scaled down TServers successfully"
					logger.Info("creating event, type: %s, reason: %s, message: %s",
						corev1.EventTypeNormal, scaledDownTServersEventReason, msg)
					r.recorder.Event(cluster, corev1.EventTypeNormal, scaledDownTServersEventReason, msg)
				}
			}

			// Update the Status after updating the STS
			tserverScaleCond := yugabytecomv1alpha1.YBClusterCondition{
				Type:    scalingDownTServersCondition,
				Status:  corev1.ConditionFalse,
				Reason:  noScaleDownInProgress,
				Message: noScaleDownInProgressMsg,
			}
			logger.Info("updating Status condition", "Type", tserverScaleCond.Type, "Status", tserverScaleCond.Status)
			cluster.Status.Conditions = append(cluster.Status.Conditions, tserverScaleCond)
			if err := r.Status().Update(context.TODO(), cluster); err != nil {
				return result, err
			}
		} else {
			msg := fmt.Sprintf("Scaling down TServers to %d: waiting for data move to complete",
				cluster.Status.TargetedTServerReplicas)
			logger.Info("creating event, type: %s, reason: %s, message: %s",
				corev1.EventTypeWarning, scalingDownTServersEventReason, msg)
			r.recorder.Event(cluster, corev1.EventTypeWarning, scalingDownTServersEventReason, msg)

			// Requeue with exponential back-off
			result.Requeue = true
			return result, nil
		}
	}

	return result, nil
}

func validateCR(spec *yugabytecomv1alpha1.YBClusterSpec) error {
	if spec.ReplicationFactor < 1 {
		return fmt.Errorf("Replication factor must be greater than 0. Found %d", spec.ReplicationFactor)
	}

	if &spec.TLS != nil && spec.TLS.Enabled {
		if &spec.TLS.RootCA == nil {
			return fmt.Errorf("root certificate and key are required when TLS encryption is enabled")
		} else if &spec.TLS.RootCA.Cert == nil || len(spec.TLS.RootCA.Cert) == 0 {
			return fmt.Errorf("root certificate is required when TLS encryption is enabled")
		} else if &spec.TLS.RootCA.Key == nil || len(spec.TLS.RootCA.Key) == 0 {
			return fmt.Errorf("root key is required when TLS encryption is enabled")
		}
	}

	if &spec.Master == nil {
		return fmt.Errorf("Master spec is required to create a Yugabyte DB cluster")
	}

	masterSpec := &spec.Master
	if &masterSpec.Replicas == nil {
		return fmt.Errorf("specifying Master replicas is required")
	} else if masterSpec.Replicas < 1 {
		return fmt.Errorf("master replicas must be greater than 0. Found to be %d", masterSpec.Replicas)
	}

	if &masterSpec.Storage == nil {
		return fmt.Errorf("master storage is required")
	} else if &masterSpec.Storage.Size == nil {
		return fmt.Errorf("master storage size is required")
	}

	if &spec.Tserver == nil {
		return fmt.Errorf("tserver spec is required to create a Yugabyte DB cluster")
	}

	tserverSpec := &spec.Tserver
	if &tserverSpec.Replicas == nil {
		return fmt.Errorf("specifying Master replicas is required")
	} else if tserverSpec.Replicas < 1 {
		return fmt.Errorf("master replicas must be greater than 0. Found to be %d", tserverSpec.Replicas)
	}

	if &tserverSpec.Storage == nil {
		return fmt.Errorf("tserver storage is required")
	} else if &tserverSpec.Storage.Size == nil {
		return fmt.Errorf("tserver storage size is required")
	}

	return nil
}

func addDefaults(spec *yugabytecomv1alpha1.YBClusterSpec) {
	if &spec.Image == nil {
		spec.Image = yugabytecomv1alpha1.YBImageSpec{
			Repository: imageRepositoryDefault,
			Tag:        imageTagDefault,
			PullPolicy: imagePullPolicyDefault,
		}
	} else {
		if &spec.Image.Repository == nil || len(spec.Image.Repository) == 0 {
			spec.Image.Repository = imageRepositoryDefault
		}

		if &spec.Image.Tag == nil || len(spec.Image.Tag) == 0 {
			spec.Image.Tag = imageTagDefault
		}

		if &spec.Image.PullPolicy == nil || len(spec.Image.PullPolicy) == 0 {
			spec.Image.PullPolicy = imagePullPolicyDefault
		}
	}

	if &spec.TLS == nil {
		spec.TLS = yugabytecomv1alpha1.YBTLSSpec{
			Enabled: false,
		}
	}

	if &spec.Domain == nil || len(spec.Domain) == 0 {
		spec.Domain = domainDefault
	}

	masterSpec := &spec.Master

	if &masterSpec.MasterUIPort == nil || masterSpec.MasterUIPort <= 0 {
		masterSpec.MasterUIPort = masterUIPortDefault
	}

	if &masterSpec.MasterRPCPort == nil || masterSpec.MasterRPCPort <= 0 {
		masterSpec.MasterRPCPort = masterRPCPortDefault
	}

	if &masterSpec.EnableLoadBalancer == nil {
		masterSpec.EnableLoadBalancer = false
	}

	if &masterSpec.PodManagementPolicy == nil || len(masterSpec.PodManagementPolicy) == 0 {
		masterSpec.PodManagementPolicy = podManagementPolicyDefault
	}

	if &masterSpec.Storage.Count == nil || masterSpec.Storage.Count <= 0 {
		masterSpec.Storage.Count = storageCountDefault
	}

	if &masterSpec.Storage.StorageClass == nil || len(masterSpec.Storage.StorageClass) == 0 {
		masterSpec.Storage.StorageClass = storageClassDefault
	}

	tserverSpec := &spec.Tserver

	if &tserverSpec.TserverRPCPort == nil || tserverSpec.TserverRPCPort <= 0 {
		tserverSpec.TserverRPCPort = tserverRPCPortDefault
	}

	if &tserverSpec.YCQLPort == nil || tserverSpec.YCQLPort <= 0 {
		tserverSpec.YCQLPort = ycqlPortDefault
	}

	if &tserverSpec.YedisPort == nil || tserverSpec.YedisPort <= 0 {
		tserverSpec.YedisPort = yedisPortDefault
	}

	if &tserverSpec.YSQLPort == nil || tserverSpec.YSQLPort <= 0 {
		tserverSpec.YSQLPort = ysqlPortDefault
	}

	if &tserverSpec.EnableLoadBalancer == nil ||
		(tserverSpec.EnableLoadBalancer == true &&
			(&tserverSpec.TserverUIPort == nil || tserverSpec.TserverUIPort <= 0)) {
		tserverSpec.EnableLoadBalancer = false
	}

	if &tserverSpec.PodManagementPolicy == nil || len(tserverSpec.PodManagementPolicy) == 0 {
		tserverSpec.PodManagementPolicy = appsv1.ParallelPodManagement
	}

	if &tserverSpec.Storage.Count == nil || tserverSpec.Storage.Count <= 0 {
		tserverSpec.Storage.Count = storageCountDefault
	}

	if &tserverSpec.Storage.StorageClass == nil || len(tserverSpec.Storage.StorageClass) == 0 {
		tserverSpec.Storage.StorageClass = storageClassDefault
	}
}

func createAppLabels(label string) map[string]string {
	return map[string]string{
		appLabel: label,
	}
}

func createServicePorts(cluster *yugabytecomv1alpha1.YBClusterSpec, isTServerService bool) []corev1.ServicePort {
	var servicePorts []corev1.ServicePort

	if !isTServerService {
		servicePorts = []corev1.ServicePort{
			{
				Name:       uiPortName,
				Port:       cluster.Master.MasterUIPort,
				TargetPort: intstr.FromInt(int(cluster.Master.MasterUIPort)),
			},
			{
				Name:       rpcPortName,
				Port:       cluster.Master.MasterRPCPort,
				TargetPort: intstr.FromInt(int(cluster.Master.MasterRPCPort)),
			},
		}
	} else {
		servicePorts = []corev1.ServicePort{
			{
				Name:       rpcPortName,
				Port:       cluster.Tserver.TserverRPCPort,
				TargetPort: intstr.FromInt(int(cluster.Tserver.TserverRPCPort)),
			},
			{
				Name:       ycqlPortName,
				Port:       cluster.Tserver.YCQLPort,
				TargetPort: intstr.FromInt(int(cluster.Tserver.YCQLPort)),
			},
			{
				Name:       yedisPortName,
				Port:       cluster.Tserver.YedisPort,
				TargetPort: intstr.FromInt(int(cluster.Tserver.YedisPort)),
			},
			{
				Name:       ysqlPortName,
				Port:       cluster.Tserver.YSQLPort,
				TargetPort: intstr.FromInt(int(cluster.Tserver.YSQLPort)),
			},
		}

		if cluster.Tserver.TserverUIPort > 0 {
			servicePorts = append(servicePorts, corev1.ServicePort{
				Name:       uiPortName,
				Port:       cluster.Tserver.TserverUIPort,
				TargetPort: intstr.FromInt(int(cluster.Tserver.TserverUIPort)),
			})
		}
	}

	return servicePorts
}

func createUIServicePorts(clusterSpec *yugabytecomv1alpha1.YBClusterSpec, isTServerService bool) []corev1.ServicePort {
	var servicePorts []corev1.ServicePort

	if !isTServerService {
		servicePorts = []corev1.ServicePort{
			{
				Name:       uiPortName,
				Port:       clusterSpec.Master.MasterUIPort,
				TargetPort: intstr.FromInt(int(clusterSpec.Master.MasterUIPort)),
			},
		}
	} else {
		if clusterSpec.Tserver.TserverUIPort > 0 {
			servicePorts = []corev1.ServicePort{
				{
					Name:       uiPortName,
					Port:       clusterSpec.Tserver.TserverUIPort,
					TargetPort: intstr.FromInt(int(clusterSpec.Tserver.TserverUIPort)),
				},
			}
		} else {
			servicePorts = nil
		}
	}

	return servicePorts
}

func createMasterContainerCommand(namespace string, domain string, grpcPort, replicas, replicationFactor, storageCount int32, masterGFlags []yugabytecomv1alpha1.YBGFlagSpec, tlsEnabled bool) []string {
	serviceName := masterNamePlural
	command := []string{
		"/home/yugabyte/bin/yb-master",
		fmt.Sprintf("--fs_data_dirs=%s", createListOfVolumeMountPaths(storageCount)),
		fmt.Sprintf("--rpc_bind_addresses=$(POD_NAME).%s.%s.svc.%s:%d", serviceName, namespace, domain, grpcPort),
		fmt.Sprintf("--server_broadcast_addresses=$(POD_NAME).%s.%s.svc.%s:%d", serviceName, namespace, domain, grpcPort),
		fmt.Sprintf("--master_addresses=%s", getMasterAddresses(namespace, domain, grpcPort, replicas)),
		"--use_initial_sys_catalog_snapshot=true",
		"--metric_node_name=$(POD_NAME)",
		fmt.Sprintf("--replication_factor=%d", replicationFactor),
		"--dump_certificate_entries",
		"--stderrthreshold=0",
		"--logtostderr",
	}

	if &masterGFlags != nil && len(masterGFlags) > 0 {
		for i := 0; i < len(masterGFlags); i++ {
			command = append(command, fmt.Sprintf("--%s=%s", masterGFlags[i].Key, masterGFlags[i].Value))
		}
	}

	if tlsEnabled {
		command = append(command, "--certs_dir=/opt/certs/yugabyte")
	}

	return command
}

func getMasterAddresses(namespace string, domain string, masterGRPCPort, masterReplicas int32) string {
	masters := make([]string, masterReplicas)

	for i := 0; i < int(masterReplicas); i++ {
		masters[i] = fmt.Sprintf("%s-%d.%s.%s.svc.%s:%d", masterName, i, masterNamePlural, namespace, domain, masterGRPCPort)
	}

	return strings.Join(masters, ",")
}

func createTServerContainerCommand(namespace string, domain string, masterGRPCPort, tserverGRPCPort, pgsqlPort, masterReplicas, storageCount int32, tserverGFlags []yugabytecomv1alpha1.YBGFlagSpec, tlsEnabled bool) []string {
	serviceName := tserverNamePlural
	command := []string{
		"/home/yugabyte/bin/yb-tserver",
		fmt.Sprintf("--fs_data_dirs=%s", createListOfVolumeMountPaths(storageCount)),
		fmt.Sprintf("--rpc_bind_addresses=$(POD_NAME).%s.%s.svc.%s:%d", serviceName, namespace, domain, tserverGRPCPort),
		fmt.Sprintf("--server_broadcast_addresses=$(POD_NAME).%s.%s.svc.%s:%d", serviceName, namespace, domain, tserverGRPCPort),
		"--start_pgsql_proxy",
		fmt.Sprintf("--pgsql_proxy_bind_address=0.0.0.0:%d", pgsqlPort),
		fmt.Sprintf("--tserver_master_addrs=%s", getMasterAddresses(namespace, domain, masterGRPCPort, masterReplicas)),
		"--metric_node_name=$(POD_NAME)",
		"--dump_certificate_entries",
		"--stderrthreshold=0",
		"--logtostderr",
	}

	if &tserverGFlags != nil && len(tserverGFlags) > 0 {
		for i := 0; i < len(tserverGFlags); i++ {
			command = append(command, fmt.Sprintf("--%s=%s", tserverGFlags[i].Key, tserverGFlags[i].Value))
		}
	}

	if tlsEnabled {
		command = append(command, "--certs_dir=/opt/certs/yugabyte")
	}

	return command
}

func createListOfVolumeMountPaths(storageCount int32) string {
	paths := make([]string, storageCount)
	for i := 0; i < int(storageCount); i++ {
		paths[i] = fmt.Sprintf("%s%d", volumeMountPath, i)
	}

	return strings.Join(paths, ",")
}

// SetupWithManager sets up the controller with the Manager.
func (r *YBClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&yugabytecomv1alpha1.YBCluster{}).
		Complete(r)
}
