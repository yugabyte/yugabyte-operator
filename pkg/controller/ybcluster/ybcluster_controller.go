package ybcluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/pkg/capnslog"
	yugabytev1alpha1 "github.com/yugabyte/yugabyte-k8s-operator/pkg/apis/yugabyte/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var logger = capnslog.NewPackageLogger("github.com/yugabyte/yugabyte-k8s-operator", "yugabyte-k8s-operator")

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
	yugabyteDBImageName         = "yugabytedb/yugabyte:1.3.1.0-b16"
	imageRepositoryDefault      = "yugabytedb/yugabyte"
	imageTagDefault             = "1.3.2.0-b19"
	imagePullPolicyDefault      = corev1.PullIfNotPresent
	podManagementPolicyDefault  = appsv1.ParallelPodManagement
	storageCountDefault         = int32(1)
	storageClassDefault         = "standard"
	labelHostname               = "kubernetes.io/hostname"
	appLabel                    = "app"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new YBCluster Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileYBCluster{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("ybcluster-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource YBCluster
	err = c.Watch(&source.Kind{Type: &yugabytev1alpha1.YBCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner YBCluster
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &yugabytev1alpha1.YBCluster{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileYBCluster implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileYBCluster{}

// ReconcileYBCluster reconciles a YBCluster object
type ReconcileYBCluster struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a YBCluster object and makes changes based on the state read
// and what is in the YBCluster.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileYBCluster) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger.Info("Reconciling YBCluster")

	// Fetch the YBCluster instance
	cluster := &yugabytev1alpha1.YBCluster{}
	err := r.client.Get(context.TODO(), request.NamespacedName, cluster)
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

	if err = r.reconcileSecrets(cluster); err != nil {
		// Error reconciling secrets - requeue the request.
		return reconcile.Result{}, err
	}

	if err = r.reconcileHeadlessServices(cluster); err != nil {
		// Error reconciling headless services - requeue the request.
		return reconcile.Result{}, err
	}

	if err = r.reconcileUIServices(cluster); err != nil {
		// Error reconciling ui services - requeue the request.
		return reconcile.Result{}, err
	}

	if err = r.reconcileStatefulsets(cluster); err != nil {
		// Error reconciling statefulsets - requeue the request.
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileYBCluster) reconcileSecrets(cluster *yugabytev1alpha1.YBCluster) error {
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
			logger.Infof("deleting master secret")
			if err := r.client.Delete(context.TODO(), masterSecret); err != nil {
				logger.Errorf("failed to delete existing master secrets object. err: %+v", err)
				return err
			}
		}

		if tserverSecret != nil {
			logger.Infof("deleting tserver secret")
			if err := r.client.Delete(context.TODO(), tserverSecret); err != nil {
				logger.Errorf("failed to delete existing tserver secrets object. err: %+v", err)
				return err
			}
		}
	} else {
		// check if object exists.Update it, if yes. Create it, if it doesn't
		if masterSecret != nil {
			// Updating
			logger.Infof("updating master secret")
			if err := updateMasterSecret(cluster, masterSecret); err != nil {
				logger.Errorf("failed to update master secrets object. err: %+v", err)
				return err
			}

			if err := r.client.Update(context.TODO(), masterSecret); err != nil {
				logger.Errorf("failed to update master secrets object. err: %+v", err)
				return err
			}
		} else {
			// Creating
			masterSecret, err := createMasterSecret(cluster)
			if err != nil {
				// Error creating master secret object
				logger.Errorf("forming master secret object failed. err: %+v", err)
				return err
			}

			logger.Infof("creating a new Secret %s for YBMasters in namespace %s", masterSecret.Name, masterSecret.Namespace)
			// Set YBCluster instance as the owner and controller for master secret
			if err := controllerutil.SetControllerReference(cluster, masterSecret, r.scheme); err != nil {
				return err
			}

			err = r.client.Create(context.TODO(), masterSecret)
			if err != nil {
				return err
			}
		}

		if tserverSecret != nil {
			// Updating
			logger.Infof("updating tserver secret")
			if err := updateTServerSecret(cluster, tserverSecret); err != nil {
				logger.Errorf("failed to update tserver secrets object. err: %+v", err)
				return err
			}

			if err := r.client.Update(context.TODO(), tserverSecret); err != nil {
				logger.Errorf("failed to update tserver secrets object. err: %+v", err)
				return err
			}
		} else {
			// Creating
			tserverSecret, err := createTServerSecret(cluster)
			if err != nil {
				// Error creating master secret object
				logger.Errorf("forming master secret object failed. err: %+v", err)
				return err
			}

			logger.Infof("creating a new Secret %s for YBTServers in namespace %s", tserverSecret.Name, tserverSecret.Namespace)
			// Set YBCluster instance as the owner and controller for tserver secret
			if err := controllerutil.SetControllerReference(cluster, tserverSecret, r.scheme); err != nil {
				return err
			}

			err = r.client.Create(context.TODO(), tserverSecret)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ReconcileYBCluster) fetchSecret(namespace string, isTserver bool) (*corev1.Secret, error) {
	name := masterSecretName

	if isTserver {
		name = tserverSecretName
	}

	found := &corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return found, nil
}

func (r *ReconcileYBCluster) reconcileHeadlessServices(cluster *yugabytev1alpha1.YBCluster) error {
	// Check if master headless service already exists
	found := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: masterNamePlural, Namespace: cluster.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		masterHeadlessService, err := createMasterHeadlessService(cluster)
		if err != nil {
			// Error creating master headless service object
			logger.Errorf("forming master headless service object failed. err: %+v", err)
			return err
		}

		// Set YBCluster instance as the owner and controller for master headless service
		if err := controllerutil.SetControllerReference(cluster, masterHeadlessService, r.scheme); err != nil {
			return err
		}
		logger.Infof("creating a new Headless Service %s for YBMasters in namespace %s", masterHeadlessService.Name, masterHeadlessService.Namespace)
		err = r.client.Create(context.TODO(), masterHeadlessService)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		logger.Info("updating master headless service")
		updateMasterHeadlessService(cluster, found)
		if err := r.client.Update(context.TODO(), found); err != nil {
			logger.Errorf("failed to update master headless service object. err: %+v", err)
			return err
		}
	}

	// Check if tserver headless service already exists
	found = &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: tserverNamePlural, Namespace: cluster.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		tserverHeadlessService, err := createTServerHeadlessService(cluster)
		if err != nil {
			// Error creating tserver headless service object
			logger.Errorf("forming tserver headless service object failed. err: %+v", err)
			return err
		}

		// Set YBCluster instance as the owner and controller for tserver headless service
		if err := controllerutil.SetControllerReference(cluster, tserverHeadlessService, r.scheme); err != nil {
			return err
		}

		logger.Infof("creating a new Headless Service %s for YBTServers in namespace %s", tserverHeadlessService.Name, tserverHeadlessService.Namespace)

		err = r.client.Create(context.TODO(), tserverHeadlessService)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		logger.Info("updating tserver headless service")
		updateTServerHeadlessService(cluster, found)
		if err := r.client.Update(context.TODO(), found); err != nil {
			logger.Errorf("failed to update tserver headless service object. err: %+v", err)
			return err
		}
	}

	return nil
}

func (r *ReconcileYBCluster) reconcileUIServices(cluster *yugabytev1alpha1.YBCluster) error {
	// Check if master ui service already exists
	found := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: masterUIServiceName, Namespace: cluster.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		masterUIService, err := createMasterUIService(cluster)
		if err != nil {
			// Error creating master ui service object
			logger.Errorf("forming master ui service object failed. err: %+v", err)
			return err
		}

		// Set YBCluster instance as the owner and controller for master ui service
		if err := controllerutil.SetControllerReference(cluster, masterUIService, r.scheme); err != nil {
			return err
		}

		logger.Infof("creating a new UI Service %s for YBMasters in namespace %s", masterUIService.Name, masterUIService.Namespace)
		err = r.client.Create(context.TODO(), masterUIService)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		logger.Info("updating master ui service")
		updateMasterUIService(cluster, found)
		if err := r.client.Update(context.TODO(), found); err != nil {
			logger.Errorf("failed to update master ui service object. err: %+v", err)
			return err
		}
	}

	// Check if tserver ui service already exists
	found = &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: tserverUIServiceName, Namespace: cluster.Namespace}, found)
	if err != nil && errors.IsNotFound(err) && cluster.Spec.Tserver.TserverUIPort > 0 {
		// Create TServer ui service, if it didn't existed.
		tserverUIService, err := createTServerUIService(cluster)
		if err != nil {
			// Error creating tserver ui service object
			logger.Errorf("forming tserver UI Service object failed. err: %+v", err)
			return err
		}

		// Set YBCluster instance as the owner and controller for tserver ui service
		if err := controllerutil.SetControllerReference(cluster, tserverUIService, r.scheme); err != nil {
			return err
		}

		logger.Infof("creating a new ui service %s for YBTServers in namespace %s", tserverUIService.Name, tserverUIService.Namespace)

		err = r.client.Create(context.TODO(), tserverUIService)
		if err != nil {
			return err
		}
	} else if err == nil && cluster.Spec.Tserver.TserverUIPort <= 0 {
		// Delete the service if it existed before & it is not needed going forward.
		logger.Info("deleting tserver ui service")
		if err := r.client.Delete(context.TODO(), found); err != nil {
			logger.Errorf("failed to delete tserver ui service object. err: %+v", err)
			return err
		}
	} else if err == nil && cluster.Spec.Tserver.TserverUIPort > 0 {
		// Update the service if it existed before & is needed in the new spec.
		logger.Info("updating tserver ui service")
		updateTServerUIService(cluster, found)
		if err := r.client.Update(context.TODO(), found); err != nil {
			logger.Errorf("failed to update tserver ui service object. err: %+v", err)
			return err
		}
	} else if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func (r *ReconcileYBCluster) reconcileStatefulsets(cluster *yugabytev1alpha1.YBCluster) error {
	// Check if master statefulset already exists
	found := &appsv1.StatefulSet{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: masterName, Namespace: cluster.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		masterStatefulset, err := createMasterStatefulset(cluster)
		if err != nil {
			// Error creating master statefulset object
			logger.Errorf("forming master statefulset object failed. err: %+v", err)
			return err
		}

		// Set YBCluster instance as the owner and controller for master statefulset
		if err := controllerutil.SetControllerReference(cluster, masterStatefulset, r.scheme); err != nil {
			return err
		}
		logger.Infof("creating a new Statefulset %s for YBMasters in namespace %s", masterStatefulset.Name, masterStatefulset.Namespace)
		err = r.client.Create(context.TODO(), masterStatefulset)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		logger.Info("updating master statefulset")
		updateMasterStatefulset(cluster, found)
		if err := r.client.Update(context.TODO(), found); err != nil {
			logger.Errorf("failed to update master statefulset object. err: %+v", err)
			return err
		}
	}

	tserverStatefulset, err := createTServerStatefulset(cluster)
	if err != nil {
		// Error creating tserver statefulset object
		logger.Errorf("forming tserver statefulset object failed. err: %+v", err)
		return err
	}

	// Set YBCluster instance as the owner and controller for tserver statefulset
	if err := controllerutil.SetControllerReference(cluster, tserverStatefulset, r.scheme); err != nil {
		return err
	}

	// Check if tserver statefulset already exists
	found = &appsv1.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: tserverStatefulset.Name, Namespace: tserverStatefulset.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Infof("creating a new Statefulset %s for YBTServers in namespace %s", tserverStatefulset.Name, tserverStatefulset.Namespace)

		err = r.client.Create(context.TODO(), tserverStatefulset)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		logger.Info("updating tserver statefulset")
		updateTServerStatefulset(cluster, found)
		if err := r.client.Update(context.TODO(), found); err != nil {
			logger.Errorf("failed to update tserver statefulset object. err: %+v", err)
			return err
		}
	}

	return nil
}

func validateCR(spec *yugabytev1alpha1.YBClusterSpec) error {
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

func addDefaults(spec *yugabytev1alpha1.YBClusterSpec) {
	if &spec.Image == nil {
		spec.Image = yugabytev1alpha1.YBImageSpec{
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
		spec.TLS = yugabytev1alpha1.YBTLSSpec{
			Enabled: false,
		}
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

func createServicePorts(cluster *yugabytev1alpha1.YBClusterSpec, isTServerService bool) []corev1.ServicePort {
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

func createUIServicePorts(clusterSpec *yugabytev1alpha1.YBClusterSpec, isTServerService bool) []corev1.ServicePort {
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

func createMasterContainerCommand(namespace string, grpcPort, replicas, replicationFactor, storageCount int32, masterGFlags []yugabytev1alpha1.YBGFlagSpec, tlsEnabled bool) []string {
	serviceName := masterNamePlural
	command := []string{
		"/home/yugabyte/bin/yb-master",
		fmt.Sprintf("--fs_data_dirs=%s", createListOfVolumeMountPaths(storageCount)),
		// fmt.Sprintf("--rpc_bind_addresses=$(POD_IP):%d", grpcPort),
		fmt.Sprintf("--rpc_bind_addresses=$(POD_NAME).%s.%s.svc.cluster.local:%d", serviceName, namespace, grpcPort),
		fmt.Sprintf("--server_broadcast_addresses=$(POD_NAME).%s.%s.svc.cluster.local:%d", serviceName, namespace, grpcPort),
		"--use_private_ip=never",
		fmt.Sprintf("--master_addresses=%s", getMasterAddresses(namespace, grpcPort, replicas)),
		"--use_initial_sys_catalog_snapshot=true",
		fmt.Sprintf("--replication_factor=%d", replicationFactor),
		fmt.Sprintf("--master_replication_factor=%d", replicas),
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

func getMasterAddresses(namespace string, masterGRPCPort, masterReplicas int32) string {
	masters := make([]string, masterReplicas)

	for i := 0; i < int(masterReplicas); i++ {
		masters[i] = fmt.Sprintf("%s-%d.%s.%s.svc.cluster.local:%d", masterName, i, masterNamePlural, namespace, masterGRPCPort)
	}

	return strings.Join(masters, ",")
}

func createTServerContainerCommand(namespace string, masterGRPCPort, tserverGRPCPort, pgsqlPort, masterReplicas, storageCount int32, tserverGFlags []yugabytev1alpha1.YBGFlagSpec, tlsEnabled bool) []string {
	serviceName := tserverNamePlural
	command := []string{
		"/home/yugabyte/bin/yb-tserver",
		fmt.Sprintf("--fs_data_dirs=%s", createListOfVolumeMountPaths(storageCount)),
		// fmt.Sprintf("--rpc_bind_addresses=$(POD_IP):%d", tserverGRPCPort),
		fmt.Sprintf("--rpc_bind_addresses=$(POD_NAME).%s.%s.svc.cluster.local:%d", serviceName, namespace, tserverGRPCPort),
		fmt.Sprintf("--server_broadcast_addresses=$(POD_NAME).%s.%s.svc.cluster.local:%d", serviceName, namespace, tserverGRPCPort),
		"--start_pgsql_proxy",
		fmt.Sprintf("--pgsql_proxy_bind_address=$(POD_IP):%d", pgsqlPort),
		"--use_private_ip=never",
		fmt.Sprintf("--tserver_master_addrs=%s", getMasterAddresses(namespace, masterGRPCPort, masterReplicas)),
		fmt.Sprintf("--tserver_master_replication_factor=%d", masterReplicas),
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
