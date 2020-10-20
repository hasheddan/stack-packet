/*
Copyright 2020 The Crossplane Authors.
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
	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProjectSpec defines the desired state of Project
type ProjectSpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	ForProvider                  ProjectParameters `json:"forProvider"`
}

// ProjectStatus defines the observed state of Project
type ProjectStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`
	AtProvider                     ProjectObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Project is a managed resource that represents a Equinix Metal Project
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="ID",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".spec.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,packet}
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec,omitempty"`
	Status ProjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProjectList contains a list of Equinix Metal Projects
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}

// ProjectParameters define the desired state of an Equinix Metal Project.
// https://metal.equinix.com/developers/api/projects/
//
// Reference values are used for optional parameters to determine if
// LateInitialization should update the parameter after creation.
type ProjectParameters struct {
	// Name is a display name for the Project
	// +required
	Name string `json:"name,omitempty"`

	// +immutable
	// OrganizationID string `json:"organizationId,omitempty"`

	// +optional
	// +immutable
	// OrganizationIDRef *runtimev1alpha1.Reference `json:"organizationIdRef,omitempty"`

	// +optional
	//OrganizationIDSelector *runtimev1alpha1.Selector `json:"organizationIdSelector,omitempty"`

	// +optional
	// PaymentMethodID string `json:"paymentMethodId,omitempty"`

	// CustomData
	// CustomData string `json:"customdata"`
}

// ProjectObservation is used to reflect in the Kubernetes API, the observed
// state of the Project resource from the Equinix Metal API.
// https://metal.equinix.com/developers/api/projects/
type ProjectObservation struct {
	// ID is the Project identifier in the Equinix Metal API
	ID string `json:"id"`

	// Name is the display name for the Equinix Metal Project
	Name string `json:"name,omitempty"`

	// CreatedAt is the time the Project was created
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`

	// UpdatedAt is the time the Project or label was last updated
	UpdatedAt *metav1.Time `json:"updatedAt,omitempty"`

	// BackendTransferEnabled
	// BackendTransfer_enabled bool `json:"backendTransferEnabled,omitempty"`
}
