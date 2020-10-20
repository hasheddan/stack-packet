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

package fake

import (
	"github.com/packethost/packngo"

	"github.com/packethost/crossplane-provider-equinix-metal/pkg/clients/project"
)

var _ project.ClientWithDefaults = &MockClient{}

// MockClient is a fake implementation of packngo.Client.
type MockClient struct {
	MockGet(sshkeyID string, *GetOptions) (*packngo.SSHKey, *packngo.Response, error)
	MockCreate(*packngo.SSHKeyCreateRequest) (*packngo.SSHKey, *packngo.Response, error)
	MockUpdate(sshkeyID string, *packngo.SSHKeyUpdateRequest) (*packngo.SSHKey, *packngo.Response, error)
	MockDelete(sshkeyID string) (*packngo.Response, error)

	MockGetProjectID  func(string) string
	MockGetFacilityID func(string) string
}

// Create calls the MockClient's MockCreate function.
func (c *MockClient) Create(createRequest *packngo.SSHKeyCreateRequest, projectID string) (*packngo.SSHKey, *packngo.Response, error) {
	return c.MockCreate(createRequest, projectID)
}

// Delete calls the MockClient's MockDelete function.
func (c *MockClient) Delete(requestID string, forceDelete bool) (*packngo.Response, error) {
	return c.MockDelete(requestID, forceDelete)
}

// Get calls the MockClient's MockGet function.
func (c *MockClient) Get(requestID string, options *packngo.GetOptions) (*packngo.SSHKey, *packngo.Response, error) {
	return c.MockGet(requestID, options)
}

// GetFacilityID calls the MockClient's MockGetFacilityID function.
func (c *MockClient) GetFacilityID(id string) string {
	return c.MockGetFacilityID(id)
}

// GetProjectID calls the MockClient's MockGetProjectID function.
func (c *MockClient) GetProjectID(id string) string {
	return c.MockGetProjectID(id)
}
