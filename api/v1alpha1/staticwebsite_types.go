package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StaticWebsiteSpec defines the desired state of StaticWebsite
type StaticWebsiteSpec struct {
	// Image is the container image for the static website
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Replicas is the number of replicas to deploy
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`

	// Resources defines the resource requirements for the container
	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`

	// Storage defines the storage configuration for the website content
	// +optional
	Storage *StorageSpec `json:"storage,omitempty"`
}

// ResourceRequirements describes the compute resource requirements.
type ResourceRequirements struct {
	// Limits describes the maximum amount of compute resources allowed.
	// +optional
	Limits ResourceList `json:"limits,omitempty"`
	// Requests describes the minimum amount of compute resources required.
	// +optional
	Requests ResourceList `json:"requests,omitempty"`
}

// ResourceList is a set of (resource name, quantity) pairs.
type ResourceList map[string]string

// StorageSpec defines the storage configuration
type StorageSpec struct {
	// Size is the size of the storage
	// +kubebuilder:validation:Required
	Size string `json:"size"`

	// StorageClassName is the storage class to use
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// MountPath is where the volume will be mounted in the container
	// +kubebuilder:default="/usr/share/nginx/html"
	MountPath string `json:"mountPath,omitempty"`
}

// StaticWebsiteStatus defines the observed state of StaticWebsite
type StaticWebsiteStatus struct {
	// Phase is the current phase of the StaticWebsite
	// +optional
	Phase string `json:"phase,omitempty"`

	// URL is the URL to access the website
	// +optional
	URL string `json:"url,omitempty"`

	// AvailableReplicas is the number of available replicas
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Available",type="integer",JSONPath=".status.availableReplicas"
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".status.url"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// StaticWebsite is the Schema for the staticwebsites API
type StaticWebsite struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StaticWebsiteSpec   `json:"spec,omitempty"`
	Status StaticWebsiteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StaticWebsiteList contains a list of StaticWebsite
type StaticWebsiteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StaticWebsite `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StaticWebsite{}, &StaticWebsiteList{})
}
