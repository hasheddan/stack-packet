/*
Copyright 2019 The Crossplane Authors.
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

package sshkey

import (
	"context"

	"github.com/packethost/packngo"

	"github.com/packethost/crossplane-provider-equinix-metal/apis/project/v1alpha1"
	"github.com/packethost/crossplane-provider-equinix-metal/pkg/clients"
)

// Client implements the Equinix Metal API methods needed to interact with
// SSHKeys for the Equinix Metal Crossplane Provider
type Client interface {
	Get(sshkeyID string, getOpts *packngo.GetOptions) (*packngo.SSHKey, *packngo.Response, error)
	Create(*packngo.SSHKeyCreateRequest) (*packngo.SSHKey, *packngo.Response, error)
	Update(sshkeyID string, updateReq *packngo.SSHKeyUpdateRequest) (*packngo.SSHKey, *packngo.Response, error)
	Delete(sshkeyID string) (*packngo.Response, error)
}

// build-time test that the interface is implemented
var _ Client = (&packngo.Client{}).SSHKeys

// ClientWithDefaults is an interface that provides SSHKeys services
// and provides default values for common properties
type ClientWithDefaults interface {
	Client
	clients.DefaultGetter
}

// CredentialedClient is a credentialed client to Equinix Metal SSHKeys
// services
type CredentialedClient struct {
	Client
	*clients.Credentials
}

var _ ClientWithDefaults = &CredentialedClient{}

// NewClient returns a Client implementing the Equinix Metal API methods needed to
// interact with SSHKeys for the Equinix Metal Crossplane Provider
func NewClient(ctx context.Context, credentials []byte, projectID string) (ClientWithDefaults, error) {
	client, err := clients.NewClient(ctx, credentials)
	if err != nil {
		return nil, err
	}
	sshkeyClient := CredentialedClient{
		Client:      client.Client.SSHKeys,
		Credentials: client.Credentials,
	}
	sshkeyClient.SetProjectID(projectID)
	return sshkeyClient, nil
}

// IsUpToDate returns true if the supplied Kubernetes resource does not differ
// from the supplied Equinix Metal resource.  It considers only fields that can be
// modified in place without deleting and recreating the instance, which are
// immutable.
func IsUpToDate(s *v1alpha1.SSHKey, p *packngo.SSHKey) bool {
	return s.Label == p.Label
}

func NewUpdateSSHKeyRequest(s) *packngo.SSHKeyUpdateRequest {
	return &packngo.SSHKeyUpdateRequest{
		Label: s.Label,
	}
}
