package ybcluster

import (
	"encoding/base64"
	"fmt"
	"strings"

	yugabytev1alpha1 "github.com/yugabyte/yugabyte-k8s-operator/pkg/apis/yugabyte/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ybClusterNamelabel is the label key for cluster name
	ybClusterNameLabel = "yugabyte.com/ybcluster-name"
)

func createMasterSecret(cluster *yugabytev1alpha1.YBCluster) (*corev1.Secret, error) {
	return createSecret(cluster, false)
}

func createTServerSecret(cluster *yugabytev1alpha1.YBCluster) (*corev1.Secret, error) {
	return createSecret(cluster, true)
}

func createSecret(cluster *yugabytev1alpha1.YBCluster, isTServerService bool) (*corev1.Secret, error) {
	if !cluster.Spec.TLS.Enabled {
		return nil, nil
	}

	secretName := masterSecretName
	label := masterName

	if isTServerService {
		secretName = tserverSecretName
		label = tserverName
	}

	secretData, err := getSecretData(cluster, isTServerService)

	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: cluster.Namespace,
			Labels:    createAppLabels(label),
		},
		Type: corev1.SecretTypeOpaque,
		Data: *secretData,
	}, nil
}

func getSecretData(cluster *yugabytev1alpha1.YBCluster, isTServerService bool) (*map[string][]byte, error) {
	data := make(map[string][]byte)
	replicas := cluster.Spec.Master.Replicas
	nodeName := masterName
	headlessServiceName := masterNamePlural

	if isTServerService {
		replicas = cluster.Spec.Tserver.Replicas
		nodeName = tserverName
		headlessServiceName = tserverNamePlural
	}

	for i := 0; i < int(replicas); i++ {
		cn := fmt.Sprintf("%s-%d.%s.%s.svc.%s", nodeName, i, headlessServiceName, cluster.Namespace, cluster.Spec.Domain)
		dnsList := []string{fmt.Sprintf("*.*.%s", cluster.Namespace), fmt.Sprintf("*.*.%s.svc.%s", cluster.Namespace, cluster.Spec.Domain)}
		certPair, err := generateSignedCerts(cn, dnsList, 3650, &cluster.Spec.TLS.RootCA)

		if err != nil {
			return nil, err
		}

		data[fmt.Sprintf("node.%s.crt", cn)] = []byte(certPair.cert)
		data[fmt.Sprintf("node.%s.key", cn)] = []byte(certPair.key)
	}
	b64decodedCert, _ := base64.StdEncoding.DecodeString(cluster.Spec.TLS.RootCA.Cert)
	data["ca.crt"] = []byte(b64decodedCert)

	return &data, nil
}

func createMasterHeadlessService(cluster *yugabytev1alpha1.YBCluster) (*corev1.Service, error) {
	return createHeadlessService(cluster, false)
}

func createTServerHeadlessService(cluster *yugabytev1alpha1.YBCluster) (*corev1.Service, error) {
	return createHeadlessService(cluster, true)
}

func createHeadlessService(cluster *yugabytev1alpha1.YBCluster, isTServerService bool) (*corev1.Service, error) {
	serviceName := masterNamePlural
	label := masterName

	if isTServerService {
		serviceName = tserverNamePlural
		label = tserverName
	}

	// This service only exists to create DNS entries for each pod in the stateful
	// set such that they can resolve each other's IP addresses. It does not
	// create a load-balanced ClusterIP and should not be used directly by clients
	// in most circumstances.
	headlessService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: cluster.Namespace,
			Labels:    createAppLabels(label),
		},
		Spec: corev1.ServiceSpec{
			Selector: createAppLabels(label),
			// We want all pods in the StatefulSet to have their addresses published for
			// the sake of the other YugabyteDB pods even before they're ready, since they
			// have to be able to talk to each other in order to become ready.
			ClusterIP: "None",
			Ports:     createServicePorts(&cluster.Spec, isTServerService),
		},
	}

	return headlessService, nil
}

func createMasterUIService(cluster *yugabytev1alpha1.YBCluster) (*corev1.Service, error) {
	return createUIService(cluster, false)
}

// Create UI service for TServer, if user has specified a UI port for it. Do not create it implicitly, with default port.
func createTServerUIService(cluster *yugabytev1alpha1.YBCluster) (*corev1.Service, error) {
	if cluster.Spec.Tserver.TserverUIPort <= 0 {
		return nil, nil
	}

	return createUIService(cluster, true)
}

func createUIService(cluster *yugabytev1alpha1.YBCluster, isTServerService bool) (*corev1.Service, error) {
	serviceName := masterUIServiceName
	label := masterName
	enableLoadBalancer := cluster.Spec.Master.EnableLoadBalancer

	if isTServerService {
		// If user hasn't specified TServer UI port, do not create a UI service for it.
		if cluster.Spec.Tserver.TserverUIPort <= 0 {
			return nil, nil
		}

		serviceName = tserverUIServiceName
		label = tserverName
		enableLoadBalancer = cluster.Spec.Tserver.EnableLoadBalancer
	}

	// This service is meant to be used by clients of the database. It exposes a ClusterIP that will
	// automatically load balance connections to the different database pods.
	uiService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: cluster.Namespace,
			Labels:    createAppLabels(label),
		},
		Spec: corev1.ServiceSpec{
			Selector: createAppLabels(label),
			Type:     corev1.ServiceTypeClusterIP,
			Ports:    createUIServicePorts(&cluster.Spec, isTServerService),
		},
	}

	if enableLoadBalancer {
		uiService.Spec.Type = corev1.ServiceTypeLoadBalancer
	}

	return uiService, nil
}

func createMasterStatefulset(cluster *yugabytev1alpha1.YBCluster) (*appsv1.StatefulSet, error) {
	return createStatefulSet(cluster, false)
}

func createTServerStatefulset(cluster *yugabytev1alpha1.YBCluster) (*appsv1.StatefulSet, error) {
	return createStatefulSet(cluster, true)
}

func createStatefulSet(cluster *yugabytev1alpha1.YBCluster, isTServerStatefulset bool) (*appsv1.StatefulSet, error) {
	replicas := cluster.Spec.Master.Replicas
	name := masterName
	label := masterName
	serviceName := masterNamePlural
	podManagementPolicy := cluster.Spec.Master.PodManagementPolicy
	volumeClaimTemplates := getVolumeClaimTemplates(&cluster.Spec.Master.Storage)

	if isTServerStatefulset {
		replicas = cluster.Spec.Tserver.Replicas
		name = tserverName
		label = tserverName
		serviceName = tserverNamePlural
		podManagementPolicy = cluster.Spec.Tserver.PodManagementPolicy
		volumeClaimTemplates = getVolumeClaimTemplates(&cluster.Spec.Tserver.Storage)
	}

	podLabels := createAppLabels(label)
	podLabels[ybClusterNameLabel] = cluster.ObjectMeta.Name

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
			Labels:    createAppLabels(label),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         serviceName,
			PodManagementPolicy: podManagementPolicy,
			Replicas:            &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: podLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: cluster.Namespace,
					Labels:    podLabels,
				},
				Spec: createPodSpec(cluster, isTServerStatefulset, name, serviceName),
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			VolumeClaimTemplates: *volumeClaimTemplates,
		},
	}

	return statefulSet, nil
}

func getVolumeClaimTemplates(storageSpec *yugabytev1alpha1.YBStorageSpec) *[]corev1.PersistentVolumeClaim {
	volumeClaimTemplates := make([]corev1.PersistentVolumeClaim, storageSpec.Count)
	for i := 0; i < int(storageSpec.Count); i++ {
		volumeClaimTemplates[i] = corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s%d", volumeMountName, i),
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(storageSpec.Size),
					},
				},
				StorageClassName: &storageSpec.StorageClass,
			},
		}
	}

	return &volumeClaimTemplates
}

func createPodSpec(cluster *yugabytev1alpha1.YBCluster, isTServerStatefulset bool, name, serviceName string) corev1.PodSpec {
	return corev1.PodSpec{
		Affinity: &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: int32(100),
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{name},
									},
								},
							},
							TopologyKey: labelHostname,
						},
					},
				},
			},
		},
		Containers: []corev1.Container{createContainer(cluster, isTServerStatefulset, name, serviceName)},
		Volumes:    getVolumes(cluster.Spec.TLS.Enabled, isTServerStatefulset),
	}
}

func createContainer(cluster *yugabytev1alpha1.YBCluster, isTServerStatefulset bool, name, serviceName string) corev1.Container {
	masterSpec := cluster.Spec.Master
	command := createMasterContainerCommand(cluster.Namespace, cluster.Spec.Domain, masterSpec.MasterRPCPort, masterSpec.Replicas, cluster.Spec.ReplicationFactor, masterSpec.Storage.Count, masterSpec.Gflags, cluster.Spec.TLS.Enabled)
	containerPorts := createMasterContainerPortsList(masterSpec.MasterUIPort, masterSpec.MasterRPCPort)
	storageSpec := &masterSpec.Storage
	resources := masterSpec.Resources

	if isTServerStatefulset {
		masterRPCPort := masterSpec.MasterRPCPort
		tserverSpec := cluster.Spec.Tserver
		command = createTServerContainerCommand(cluster.Namespace, cluster.Spec.Domain, masterRPCPort, tserverSpec.TserverRPCPort, tserverSpec.YSQLPort, masterSpec.Replicas, tserverSpec.Storage.Count, tserverSpec.Gflags, cluster.Spec.TLS.Enabled)
		containerPorts = createTServerContainerPortsList(tserverSpec.TserverUIPort, tserverSpec.TserverRPCPort, tserverSpec.YCQLPort, tserverSpec.YedisPort, tserverSpec.YSQLPort)
		storageSpec = &tserverSpec.Storage
		resources = tserverSpec.Resources
	}

	return corev1.Container{
		Name:            name,
		Image:           strings.Join([]string{cluster.Spec.Image.Repository, cluster.Spec.Image.Tag}, ":"),
		ImagePullPolicy: corev1.PullAlways,
		Env: []corev1.EnvVar{
			{
				Name:  envGetHostsFrom,
				Value: envGetHostsFromVal,
			},
			{
				Name: envPodIP,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: envPodIPVal,
					},
				},
			},
			{
				Name: envPodName,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: envPodNameVal,
					},
				},
			},
		},
		Resources:    resources,
		Command:      command,
		Ports:        containerPorts,
		VolumeMounts: getVolumeMounts(storageSpec, cluster.Spec.TLS.Enabled, isTServerStatefulset),
	}
}

func getVolumes(tlsEnabled, isTServer bool) []corev1.Volume {
	if tlsEnabled {
		mountName := masterSecretName

		if isTServer {
			mountName = tserverSecretName
		}

		return []corev1.Volume{
			{
				Name: mountName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: mountName,
					},
				},
			},
		}
	}

	return nil
}

func getVolumeMounts(storageSpec *yugabytev1alpha1.YBStorageSpec, tlsEnabled, isTServer bool) []corev1.VolumeMount {
	volumeMounts := make([]corev1.VolumeMount, storageSpec.Count)

	for i := 0; i < int(storageSpec.Count); i++ {
		volumeMounts[i] = corev1.VolumeMount{
			Name:      fmt.Sprintf("%s%d", volumeMountName, i),
			MountPath: fmt.Sprintf("%s%d", volumeMountPath, i),
		}
	}

	if tlsEnabled {
		mountName := masterSecretName

		if isTServer {
			mountName = tserverSecretName
		}

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      mountName,
			MountPath: secretMountPath,
			ReadOnly:  true,
		})
	}

	return volumeMounts
}

func createMasterContainerPortsList(masterUIPort, masterRPCPort int32) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{
		{
			Name:          masterContainerUIPortName,
			ContainerPort: masterUIPort,
		},
		{
			Name:          masterContainerRPCPortName,
			ContainerPort: masterRPCPort,
		},
	}

	return ports
}

func createTServerContainerPortsList(uiPort, rpcPort, ycqlPort, yedisPort, ysqlPort int32) []corev1.ContainerPort {
	tserverUIPort := uiPort

	if tserverUIPort <= 0 {
		tserverUIPort = tserverUIPortDefault
	}

	ports := []corev1.ContainerPort{
		{
			Name:          tserverContainerUIPortName,
			ContainerPort: tserverUIPort,
		},
		{
			Name:          tserverContainerRPCPortName,
			ContainerPort: rpcPort,
		},
		{
			Name:          ycqlPortName,
			ContainerPort: ycqlPort,
		},
		{
			Name:          yedisPortName,
			ContainerPort: yedisPort,
		},
		{
			Name:          ysqlPortName,
			ContainerPort: ysqlPort,
		},
	}

	return ports
}
