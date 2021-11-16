package controllers

import (
	yugabytecomv1alpha1 "github.com/yugabyte/yugabyte-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func updateMasterSecret(newCluster *yugabytecomv1alpha1.YBCluster, secret *corev1.Secret) error {
	return updateSecret(newCluster, secret, false)
}

func updateTServerSecret(newCluster *yugabytecomv1alpha1.YBCluster, secret *corev1.Secret) error {
	return updateSecret(newCluster, secret, true)
}

func updateSecret(newCluster *yugabytecomv1alpha1.YBCluster, secret *corev1.Secret, isTServerService bool) error {
	secretData, err := getSecretData(newCluster, isTServerService)

	if err != nil {
		return err
	}

	secret.Data = *secretData

	return nil
}

func updateMasterHeadlessService(newCluster *yugabytecomv1alpha1.YBCluster, existingService *corev1.Service) {
	updateHeadlessService(newCluster, existingService, false)
}

func updateTServerHeadlessService(newCluster *yugabytecomv1alpha1.YBCluster, existingService *corev1.Service) {
	updateHeadlessService(newCluster, existingService, true)
}

func updateHeadlessService(newCluster *yugabytecomv1alpha1.YBCluster, existingService *corev1.Service, isTServerService bool) {
	existingService.Spec.Ports = createServicePorts(&newCluster.Spec, isTServerService)
}

func updateMasterUIService(newCluster *yugabytecomv1alpha1.YBCluster, existingService *corev1.Service) {
	updateUIService(newCluster, existingService, false)
}

// Update/Delete UI service for TServer, if user has specified/removed a UI port for it.
func updateTServerUIService(newCluster *yugabytecomv1alpha1.YBCluster, existingService *corev1.Service) {
	updateUIService(newCluster, existingService, true)
}

func updateUIService(newCluster *yugabytecomv1alpha1.YBCluster, existingService *corev1.Service, isTServerService bool) {
	// Update the UI service for Master or TServer, otherwise.
	existingService.Spec.Ports = createUIServicePorts(&newCluster.Spec, isTServerService)
}

func updateMasterStatefulset(newCluster *yugabytecomv1alpha1.YBCluster, sfs *appsv1.StatefulSet) error {
	return updateStatefulSet(newCluster, sfs, false)
}

func updateTServerStatefulset(newCluster *yugabytecomv1alpha1.YBCluster, sfs *appsv1.StatefulSet) error {
	return updateStatefulSet(newCluster, sfs, true)
}

func updateStatefulSet(newCluster *yugabytecomv1alpha1.YBCluster, sfs *appsv1.StatefulSet, isTServerStatefulset bool) error {
	masterSpec := newCluster.Spec.Master
	replicas := masterSpec.Replicas
	masterRPCPort := masterSpec.MasterRPCPort
	volumeClaimTemplates := getVolumeClaimTemplates(&masterSpec.Storage)
	command := createMasterContainerCommand(newCluster.Namespace, newCluster.Spec.Domain, masterRPCPort, replicas, newCluster.Spec.ReplicationFactor, masterSpec.Storage.Count, masterSpec.Gflags, newCluster.Spec.TLS.Enabled)
	containerPorts := createMasterContainerPortsList(masterSpec.MasterUIPort, masterRPCPort)
	storageSpec := &masterSpec.Storage

	if isTServerStatefulset {
		tserverSpec := newCluster.Spec.Tserver
		replicas = newCluster.Status.TargetedTServerReplicas
		volumeClaimTemplates = getVolumeClaimTemplates(&tserverSpec.Storage)
		command = createTServerContainerCommand(newCluster.Namespace, newCluster.Spec.Domain, masterRPCPort, tserverSpec.TserverRPCPort, tserverSpec.YSQLPort, masterSpec.Replicas, tserverSpec.Storage.Count, tserverSpec.Gflags, newCluster.Spec.TLS.Enabled)
		containerPorts = createTServerContainerPortsList(tserverSpec.TserverUIPort, tserverSpec.TserverRPCPort, tserverSpec.YCQLPort, tserverSpec.YedisPort, tserverSpec.YSQLPort)
		storageSpec = &tserverSpec.Storage
	}

	sfs.Spec.Replicas = &replicas
	sfs.Spec.Template.Spec.Containers[0].Command = command
	sfs.Spec.Template.Spec.Containers[0].Ports = containerPorts
	sfs.Spec.Template.Spec.Containers[0].VolumeMounts = getVolumeMounts(storageSpec, newCluster.Spec.TLS.Enabled, isTServerStatefulset)
	sfs.Spec.VolumeClaimTemplates = *volumeClaimTemplates

	return nil
}
