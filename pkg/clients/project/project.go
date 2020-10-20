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

package project

import (
	"context"

	"github.com/packethost/packngo"

	"github.com/packethost/crossplane-provider-equinix-metal/apis/project/v1alpha1"
	"github.com/packethost/crossplane-provider-equinix-metal/pkg/clients"
)

// Client implements the Equinix Metal API methods needed to interact with
// Projects for the Equinix Metal Crossplane Provider
type Client interface {
	Get(projectID string, getOpts *packngo.GetOptions) (*packngo.Project, *packngo.Response, error)
	Create(*packngo.ProjectCreateRequest) (*packngo.Project, *packngo.Response, error)
	Update(projectID string, updateReq *packngo.ProjectUpdateRequest) (*packngo.Project, *packngo.Response, error)
	Delete(projectID string) (*packngo.Response, error)
}

// build-time test that the interface is implemented
var _ Client = (&packngo.Client{}).Projects

// ClientWithDefaults is an interface that provides Project services
// and provides default values for common properties
type ClientWithDefaults interface {
	Client
	clients.DefaultGetter
}

// CredentialedClient is a credentialed client to Equinix Metal Projects
// services
type CredentialedClient struct {
	Client
	*clients.Credentials
}

var _ ClientWithDefaults = &CredentialedClient{}

// NewClient returns a Client implementing the Equinix Metal API methods needed to
// interact with Projects for the Equinix Metal Crossplane Provider
func NewClient(ctx context.Context, credentials []byte, projectID string) (ClientWithDefaults, error) {
	client, err := clients.NewClient(ctx, credentials)
	if err != nil {
		return nil, err
	}
	projectClient := CredentialedClient{
		Client:      client.Client.Projects,
		Credentials: client.Credentials,
	}
	projectClient.SetProjectID(projectID)
	return projectClient, nil
}

// IsUpToDate returns true if the supplied Kubernetes resource does not differ
// from the supplied Equinix Metal resource.  It considers only fields that can be
// modified in place without deleting and recreating the instance, which are
// immutable.
func IsUpToDate(s *v1alpha1.Project, p *packngo.Project) bool {
	return s.Label == p.Name
}

func NewUpdateProjectRequest(p *v1alpha1.Project) *packngo.ProjectUpdateRequest {
	return &packngo.ProjectUpdateRequest{
		Name: p.Label,
	}
}
