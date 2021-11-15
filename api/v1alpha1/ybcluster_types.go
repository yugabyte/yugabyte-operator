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

package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// YBClusterSpec defines the desired state of YBCluster
// +k8s:openapi-gen=true
type YBClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// +kubebuilder:validation:Minimum=1
	ReplicationFactor int32 `json:"replicationFactor,omitempty"`
	// Domain name for Kubernetes cluster
	Domain  string        `json:"domain,omitempty"`
	Image   YBImageSpec   `json:"image,omitempty"`
	TLS     YBTLSSpec     `json:"tls,omitempty"`
	Master  YBMasterSpec  `json:"master,omitempty"`
	Tserver YBTServerSpec `json:"tserver,omitempty"`
}

// YBImageSpec defines docker image specific attributes.
// +k8s:openapi-gen=true
type YBImageSpec struct {
	Repository string        `json:"repository,omitempty"`
	Tag        string        `json:"tag,omitempty"`
	PullPolicy v1.PullPolicy `json:"pullPolicy,omitempty"`
}

// YBTLSSpec defines TLS encryption specific attributes
// +k8s:openapi-gen=true
type YBTLSSpec struct {
	Enabled bool         `json:"enabled,omitempty"`
	RootCA  YBRootCASpec `json:"rootCA,omitempty"`
}

// YBRootCASpec defines Root CA cert & key attributes required for enabling TLS encryption.
// +k8s:openapi-gen=true
type YBRootCASpec struct {
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

// YBMasterSpec defines attributes for YBMaster pods.
// +k8s:openapi-gen=true
type YBMasterSpec struct {
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas,omitempty"`
	// +kubebuilder:validation:Minimum=1
	MasterUIPort int32 `json:"masterUIPort,omitempty"`
	// +kubebuilder:validation:Minimum=1
	MasterRPCPort       int32                          `json:"masterRPCPort,omitempty"`
	EnableLoadBalancer  bool                           `json:"enableLoadBalancer,omitempty"`
	PodManagementPolicy appsv1.PodManagementPolicyType `json:"podManagementPolicy,omitempty"`
	Storage             YBStorageSpec                  `json:"storage,omitempty"`
	Resources           v1.ResourceRequirements        `json:"resources,omitempty"`
	// +kubebuilder:validation:MinItems=1
	Gflags []YBGFlagSpec `json:"gflags,omitempty"`
}

// YBTServerSpec defines attributes for YBTServer pods.
// +k8s:openapi-gen=true
type YBTServerSpec struct {
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas,omitempty"`
	// +kubebuilder:validation:Minimum=1
	TserverUIPort int32 `json:"tserverUIPort,omitempty"`
	// +kubebuilder:validation:Minimum=1
	TserverRPCPort int32 `json:"tserverRPCPort,omitempty"`
	// +kubebuilder:validation:Minimum=1
	YCQLPort int32 `json:"ycqlPort,omitempty"`
	// +kubebuilder:validation:Minimum=1
	YedisPort int32 `json:"yedisPort,omitempty"`
	// +kubebuilder:validation:Minimum=1
	YSQLPort            int32                          `json:"ysqlPort,omitempty"`
	EnableLoadBalancer  bool                           `json:"enableLoadBalancer,omitempty"`
	PodManagementPolicy appsv1.PodManagementPolicyType `json:"podManagementPolicy,omitempty"`
	Storage             YBStorageSpec                  `json:"storage,omitempty"`
	Resources           v1.ResourceRequirements        `json:"resources,omitempty"`
	// +kubebuilder:validation:MinItems=1
	Gflags []YBGFlagSpec `json:"gflags,omitempty"`
}

// YBStorageSpec defines storage specific attributes for YBMaster/YBTserver pods.
// +k8s:openapi-gen=true
type YBStorageSpec struct {
	// +kubebuilder:validation:Minimum=1
	Count int32 `json:"count,omitempty"`
	// +kubebuilder:validation:Pattern=`^[0-9]{1,4}[MGT][IBib]$`
	Size         string `json:"size,omitempty"`
	StorageClass string `json:"storageClass,omitempty"`
}

// YBGFlagSpec defines key-value pairs for each GFlag.
// +k8s:openapi-gen=true
type YBGFlagSpec struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// YBClusterStatus defines the observed state of YBCluster
// +k8s:openapi-gen=true
type YBClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	MasterReplicas  int64 `json:"masterReplicas"`
	TserverReplicas int64 `json:"tserverReplicas"`

	// TargetedTServerReplicas is the desired number of replicas
	// currently targeted. If any other operation is going on,
	// then change in spec.tserver.replicas won't modify this
	// value until the operation is completed.
	TargetedTServerReplicas int32 `json:"targetedTServerReplicas"`

	// Conditions represent the latest available observations of an object's state
	Conditions []YBClusterCondition `json:"conditions,omitempty"`
}

type YBClusterCondition struct {
	Type               string                 `json:"type"`
	Status             corev1.ConditionStatus `json:"status"`
	Reason             string                 `json:"reason,omitempty"`
	Message            string                 `json:"message,omitempty"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// YBCluster is the Schema for the ybclusters API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=ybclusters,scope=Namespaced
type YBCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   YBClusterSpec   `json:"spec,omitempty"`
	Status YBClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// YBClusterList contains a list of YBCluster
type YBClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []YBCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&YBCluster{}, &YBClusterList{})
}
