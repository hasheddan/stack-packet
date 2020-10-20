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

// SSHKeySpec defines the desired state of SSHKey
type SSHKeySpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	ForProvider                  SSHKeyParameters `json:"forProvider"`
}

// SSHKeyStatus defines the observed state of SSHKey
type SSHKeyStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`
	AtProvider                     SSHKeyObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// SSHKey is a managed resource that represents a Equinix Metal Project SSH key
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="ID",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".spec.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,packet}
type SSHKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SSHKeySpec   `json:"spec,omitempty"`
	Status SSHKeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SSHKeyList contains a list of Project SSH keys
type SSHKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SSHKey `json:"items"`
}

// SSHKeyParameters define the desired state of an Equinix Metal Virtual Network.
// https://metal.equinix.com/developers/api/sshkeys/#create-a-ssh-key-for-the-given-project
//
// Reference values are used for optional parameters to determine if
// LateInitialization should update the parameter after creation.
type SSHKeyParameters struct {
	// +immutable
	ProjectID string `json:"projectId,omitempty"`

	// +optional
	// +immutable
	ProjectIDRef *runtimev1alpha1.Reference `json:"projectIdRef,omitempty"`

	// +optional
	ProjectIDSelector *runtimev1alpha1.Selector `json:"projectIdSelector,omitempty"`

	// Key is the public SSH key (example: "ssh-rsa AAAA... user@host")
	Key string `json:"key"`

	// Label is a display name for the SSH key
	Label string `json:"label,omitempty"`
}

// SSHKeyObservation is used to reflect in the Kubernetes API, the observed
// state of the ProjectSSHKey resource from the Equinix Metal API.
// https://metal.equinix.com/developers/api/sshkeys/#retrieve-a-devices-ssh-keys
type SSHKeyObservation struct {
	// ID is the SSH key identifier in the Equinix Metal API
	ID string `json:"id"`

	// Href is the canonical Equinix Metal API URL reference for this resource
	Href string `json:"href,omitempty"`

	// Fingerprint is the SSH public key fingerprint
	Fingerprint string `json:"fingerprint,omitempty"`

	// CreatedAt is the time the SSH key was created
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`

	// UpdatedAt is the time the SSH key or label was last updated
	UpdatedAt *metav1.Time `json:"updatedAt,omitempty"`

	// Label is a display name for the SSH key
	Label string `json:"label,omitempty"`

	// Key is the public SSH key (example: "ssh-rsa AAAA... user@host")
	Key string `json:"key,omitempty"`

	// TODO(displague) should we include Owner?
}
