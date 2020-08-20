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

// RequestSpec defines the desired state of Request
type RequestSpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	ForProvider                  RequestParameters `json:"forProvider"`
}

// RequestStatus defines the observed state of Request
type RequestStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`
	AtProvider                     RequestObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Request is a managed resource that represents a Packet Request
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="ID",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".spec.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,packet}
type Request struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RequestSpec   `json:"spec,omitempty"`
	Status RequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RequestList contains a list of Requests
type RequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Request `json:"items"`
}

// RequestParameters define the desired state of a Packet Virtual Network.
// https://www.packet.com/developers/api/vlans/#create-an-virtual-network
//
// Reference values are used for optional parameters to determine if
// LateInitialization should update the parameter after creation.
type RequestParameters struct {
	DevicesMax int          `json:"devicesMax"`
	DevicesMin int          `json:"devicesMin"`
	EndAt      *metav1.Time `json:"endAt,omitempty"`
	Facilities []string     `json:"facilities"`

	// MaxBidPrice the maximum price you are willing to pay for the
	// instance, per hour. This should be greater or equal than the current spot
	// price for the given facility and plan.
	MaxBidPrice string `json:"maxBidPrice"`

	Attributes RequestInstanceAttributes `json:"attributes"`
}

// RequestInstanceAttributes specify how devices fulfilled by a
// SpotMarketRequest bid will be created. This is similar to DeviceParameters.
type RequestInstanceAttributes struct {
	AlwaysPXE       bool     `json:"alwaysPXE,omitempty"`
	BillingCycle    string   `json:"billingCycle"`
	CustomData      string   `json:"customData,omitempty"`
	Description     string   `json:"description,omitempty"`
	Features        []string `json:"features,omitempty"`
	Hostname        string   `json:"hostname,omitempty"`
	Hostnames       []string `json:"hostnames,omitempty"`
	Locked          bool     `json:"locked,omitempty"`
	OperatingSystem string   `json:"operatingSystem"`
	Plan            string   `json:"plan"`
	ProjectSSHKeys  []string `json:"projectSSHKeys,omitempty"`
	Tags            []string `json:"tags"`

	// TerminationTime is an optional fixed date and time [UTC] in the future at
	// which to terminate the instance. This does not guarantee your instance
	// will run until that time as we may need to revoke the instance. By
	// default the system will terminate an instance at 120 seconds if the
	// resource is required.
	TerminationTime *metav1.Time `json:"terminationTime,omitempty"`
	UserSSHKeys     []string     `json:"userSSHKeys,omitempty"`
	UserData        string       `json:"userData"`
}

// RequestObservation is used to reflect in the Kubernetes API, the observed
// state of the SpotMarketRequest resource from the Packet API.
type RequestObservation struct {
	ID        string       `json:"id"`
	Href      string       `json:"href,omitempty"`
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`

	DeviceIDs []string `json:"deviceIds,omitempty"`
}
