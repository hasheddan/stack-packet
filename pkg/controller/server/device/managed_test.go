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

package device

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/packethost/packngo"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/packethost/crossplane-provider-equinix-metal/apis/server/v1alpha2"
	packetv1beta1 "github.com/packethost/crossplane-provider-equinix-metal/apis/v1beta1"
	"github.com/packethost/crossplane-provider-equinix-metal/pkg/clients"
	devicesclient "github.com/packethost/crossplane-provider-equinix-metal/pkg/clients/device"
	"github.com/packethost/crossplane-provider-equinix-metal/pkg/clients/device/fake"
	packettest "github.com/packethost/crossplane-provider-equinix-metal/pkg/test"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

const (
	namespace  = "cool-namespace"
	deviceName = "my-cool-device"

	providerName       = "cool-equinix-metal"
	providerSecretName = "cool-equinix-metal-secret"
	providerSecretKey  = "credentials"
	providerSecretData = "{\"definitely\":\"json\"}"

	connectionSecretName = "cool-connection-secret"
)

var (
	errorBoom = errors.New("boom")

	// Use layer2-individual as the default, empty packngo.Device{} will
	// self-detect as layer2-individual based on port and bonding configuration.
	// layer3, is the default for real new devices.
	networkType = packngo.NetworkTypeL2Individual

	truthy    = true
	alwaysPXE = &truthy

	// mockNetworkTypeConfigs provides easy mocking for NetworkType.
	// NetworkType is computed from port, bonding, and IP configuration
	// test values are provided for easy mocking
	mockNetworkTypeConfigs = map[string]struct {
		NetworkPorts []packngo.Port
		Network      []*packngo.IPAddressAssignment
	}{
		packngo.NetworkTypeL2Bonded: {
			NetworkPorts: []packngo.Port{{
				Name:        "bond0",
				Type:        "NetworkBondPort",
				NetworkType: networkType,
				Data:        packngo.PortData{Bonded: true},
			},
				{
					Name: "eth0",
					Type: "NetworkPort",
					Data: packngo.PortData{Bonded: true},
					Bond: &packngo.BondData{Name: "bond0"},
				}},
			Network: []*packngo.IPAddressAssignment{{
				IpAddressCommon: packngo.IpAddressCommon{
					Management: false,
				},
			}},
		},

		packngo.NetworkTypeL3: {
			NetworkPorts: []packngo.Port{{
				Name:        "bond0",
				Type:        "NetworkBondPort",
				NetworkType: networkType,
				Data:        packngo.PortData{Bonded: true},
			},
				{
					Name: "eth0",
					Type: "NetworkPort",
					Data: packngo.PortData{Bonded: true},
					Bond: &packngo.BondData{Name: "bond0"},
				}},
			Network: []*packngo.IPAddressAssignment{{
				IpAddressCommon: packngo.IpAddressCommon{
					Management: true,
				},
			}},
		},

		packngo.NetworkTypeHybrid: {
			NetworkPorts: []packngo.Port{{
				Name:        "bond0",
				Type:        "NetworkBondPort",
				NetworkType: networkType,
				Data:        packngo.PortData{Bonded: true},
			},
				{
					Name: "eth0",
					Type: "NetworkPort",
					Data: packngo.PortData{Bonded: true},
					Bond: &packngo.BondData{Name: "bond0"},
				},
				{
					Name: "eth1",
					Type: "NetworkPort",
					Data: packngo.PortData{Bonded: false},
				}},
			Network: []*packngo.IPAddressAssignment{{
				IpAddressCommon: packngo.IpAddressCommon{
					Management: true,
				},
			}},
		},
	}
)

type strange struct {
	resource.Managed
}

type deviceModifier func(*v1alpha2.Device)

func withConditions(c ...xpv1.Condition) deviceModifier {
	return func(i *v1alpha2.Device) { i.Status.SetConditions(c...) }
}

func withProvisionPer(p float32) deviceModifier {
	return func(i *v1alpha2.Device) {
		i.Status.AtProvider.ProvisionPercentage = apiresource.MustParse(fmt.Sprintf("%.6f", p))
	}
}

func withState(s string) deviceModifier {
	return func(i *v1alpha2.Device) { i.Status.AtProvider.State = s }
}

func withID(d string) deviceModifier {
	return func(i *v1alpha2.Device) { i.Status.AtProvider.ID = d }
}

func withNetworkType(d *string) deviceModifier {
	return func(i *v1alpha2.Device) { i.Spec.ForProvider.NetworkType = d }
}

type initializerParams struct {
	hostname, billingCycle, userdata, ipxeScriptURL string
	locked                                          bool
}

func withInitializerParams(p initializerParams) deviceModifier {
	return func(i *v1alpha2.Device) {
		i.Spec.ForProvider.Hostname = &p.hostname
		i.Spec.ForProvider.BillingCycle = &p.billingCycle
		i.Spec.ForProvider.UserData = &p.userdata
		i.Spec.ForProvider.IPXEScriptURL = &p.ipxeScriptURL
		i.Spec.ForProvider.Locked = &p.locked
	}
}

func device(im ...deviceModifier) *v1alpha2.Device {
	i := &v1alpha2.Device{
		ObjectMeta: metav1.ObjectMeta{
			Name:       deviceName,
			Finalizers: []string{},
			Annotations: map[string]string{
				meta.AnnotationKeyExternalName: deviceName,
			},
		},
		Spec: v1alpha2.DeviceSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: providerName},
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Namespace: namespace,
					Name:      connectionSecretName,
				},
			},
			ForProvider: v1alpha2.DeviceParameters{
				AlwaysPXE: alwaysPXE,
			},
		},
	}

	for _, m := range im {
		m(i)
	}

	return i
}

func projectIDFromCredentials(_ string) string {
	return "id-from-credentials"
}

var _ managed.ExternalClient = &external{}
var _ managed.ExternalConnecter = &connecter{}

func TestConnect(t *testing.T) {
	provider := packetv1beta1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{Name: providerName},
		Spec: packetv1beta1.ProviderConfigSpec{
			Credentials: packetv1beta1.ProviderCredentials{
				Source: xpv1.CredentialsSourceSecret,
				CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
					SecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Namespace: namespace,
							Name:      providerSecretName,
						},
						Key: providerSecretKey,
					},
				},
			},
		},
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: providerSecretName},
		Data:       map[string][]byte{providerSecretKey: []byte(providerSecretData)},
	}

	type strange struct {
		resource.Managed
	}

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		err error
	}

	// copied from crossplane-runtime providerconfig.go
	const (
		errSecretKeyNotSpecified   = "cannot extract from secret key when none specified"
		errGetCredentialsSecret    = "cannot get credentials secret"
		errGetCredentials          = "cannot get credentials"
		errGetProviderConfigSecret = "cannot get ProviderConfig Secret"
		errGetProviderConfigUsage  = "cannot apply ProviderConfigUsage"
		errGetObject               = "cannot get object"
	)

	cases := map[string]struct {
		conn managed.ExternalConnecter
		args args
		want want
	}{
		"Connected": {
			conn: &connecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
					switch key {
					case client.ObjectKey{Name: providerName}:
						*obj.(*packetv1beta1.ProviderConfig) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = secret
					}
					return nil
				}},
				usage: resource.NewProviderConfigUsageTracker(&test.MockClient{
					MockGet:    test.NewMockGetFn(nil),
					MockUpdate: test.NewMockUpdateFn(nil),
				}, &packetv1beta1.ProviderConfigUsage{}),
				newClientFn: func(_ context.Context, _ *clients.Credentials) (devicesclient.ClientWithDefaults, error) {
					return nil, nil
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  device(),
			},
			want: want{
				err: nil,
			},
		},
		"NotDevice": {
			conn: &connecter{},
			args: args{ctx: context.Background(), mg: &strange{}},
			want: want{err: errors.New(errNotDevice)},
		},
		"FailedToGetProvider": {
			conn: &connecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
					return errorBoom
				}},
				usage: resource.NewProviderConfigUsageTracker(&test.MockClient{
					MockGet:    test.NewMockGetFn(nil),
					MockUpdate: test.NewMockUpdateFn(nil),
				}, &packetv1beta1.ProviderConfigUsage{}),
			},
			args: args{ctx: context.Background(), mg: device()},
			want: want{err: errors.Wrap(
				errors.Wrap(errors.Wrap(errorBoom, errGetObject), errGetProviderConfigUsage), errGetProviderConfigSecret,
			)},
		},
		"FailedToGetProviderSecret": {
			conn: &connecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
					switch key {
					case client.ObjectKey{Name: providerName}:
						*obj.(*packetv1beta1.ProviderConfig) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						return errorBoom
					}
					return nil
				}},
				usage: resource.NewProviderConfigUsageTracker(&test.MockClient{
					MockGet:    test.NewMockGetFn(nil),
					MockUpdate: test.NewMockUpdateFn(nil),
				}, &packetv1beta1.ProviderConfigUsage{}),
			},
			args: args{ctx: context.Background(), mg: device()},
			want: want{err: errors.Wrap(
				errors.Wrap(errors.Wrap(errorBoom, errGetCredentialsSecret), errGetCredentials), errGetProviderConfigSecret,
			)},
		},
		"ProviderSecretNil": {
			conn: &connecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
					switch key {
					case client.ObjectKey{Name: providerName}:
						nilSecretProvider := provider
						nilSecretProvider.Spec.Credentials.SecretRef = nil
						*obj.(*packetv1beta1.ProviderConfig) = nilSecretProvider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						return errorBoom
					}
					return nil
				}},
				usage: resource.NewProviderConfigUsageTracker(&test.MockClient{
					MockGet:    test.NewMockGetFn(nil),
					MockUpdate: test.NewMockUpdateFn(nil),
				}, &packetv1beta1.ProviderConfigUsage{}),
			},
			args: args{ctx: context.Background(), mg: device()},
			want: want{err: errors.Wrap(
				errors.Wrap(errors.New(errSecretKeyNotSpecified), errGetCredentials), errGetProviderConfigSecret,
			)},
		},
		"FailedToCreateDevice": {
			conn: &connecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
					switch key {
					case client.ObjectKey{Name: providerName}:
						*obj.(*packetv1beta1.ProviderConfig) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = secret
					}
					return nil
				}},
				usage: resource.NewProviderConfigUsageTracker(&test.MockClient{
					MockGet:    test.NewMockGetFn(nil),
					MockUpdate: test.NewMockUpdateFn(nil),
				}, &packetv1beta1.ProviderConfigUsage{}),
				newClientFn: func(_ context.Context, _ *clients.Credentials) (devicesclient.ClientWithDefaults, error) {
					return nil, errorBoom
				},
			},
			args: args{ctx: context.Background(), mg: device()},
			want: want{err: errors.Wrap(errorBoom, errNewClient)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tc.conn.Connect(tc.args.ctx, tc.args.mg)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.conn.Connect(...): want error != got error:\n%s", diff)
			}
		})
	}
}

func TestObserve(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg          resource.Managed
		observation managed.ExternalObservation
		err         error
	}

	cases := map[string]struct {
		client managed.ExternalClient
		args   args
		want   want
	}{
		"ObservedDeviceAvailableNoUpdateNeeded": {
			client: &external{
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				client: &fake.MockClient{
					MockGet: func(deviceID string, getOpt *packngo.GetOptions) (*packngo.Device, *packngo.Response, error) {
						d := &packngo.Device{
							State:        v1alpha2.StateActive,
							ProvisionPer: float32(100),
							AlwaysPXE:    *alwaysPXE,
						}
						return d, nil, nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  device(),
			},
			want: want{
				mg: device(
					withInitializerParams(initializerParams{}),
					withConditions(xpv1.Available()),
					withProvisionPer(float32(100)),
					withNetworkType(&networkType),
					withState(v1alpha2.StateActive)),
				observation: managed.ExternalObservation{
					ResourceExists:    true,
					ResourceUpToDate:  true,
					ConnectionDetails: managed.ConnectionDetails{},
				},
			},
		},
		"ObservedDeviceAvailableUpdateNeeded": {
			client: &external{
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				client: &fake.MockClient{
					MockGet: func(deviceID string, getOpt *packngo.GetOptions) (*packngo.Device, *packngo.Response, error) {
						d := &packngo.Device{
							State:        v1alpha2.StateActive,
							ProvisionPer: float32(100),
							AlwaysPXE:    !*alwaysPXE,
						}
						return d, nil, nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  device(),
			},
			want: want{
				mg: device(
					withInitializerParams(initializerParams{}),
					withConditions(xpv1.Available()),
					withProvisionPer(float32(100)),
					withNetworkType(&networkType),
					withState(v1alpha2.StateActive)),
				observation: managed.ExternalObservation{
					ResourceExists:    true,
					ResourceUpToDate:  false,
					ConnectionDetails: managed.ConnectionDetails{},
				},
			},
		},
		"ObservedDeviceCreating": {
			client: &external{
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				client: &fake.MockClient{
					MockGet: func(deviceID string, getOpt *packngo.GetOptions) (*packngo.Device, *packngo.Response, error) {
						d := &packngo.Device{
							State:        v1alpha2.StateProvisioning,
							ProvisionPer: float32(50),
							AlwaysPXE:    *alwaysPXE,
						}
						return d, nil, nil
					},
					MockDeviceNetworkType: func(_ string) (string, error) {
						return networkType, nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  device(),
			},
			want: want{
				mg: device(
					withInitializerParams(initializerParams{}),
					withConditions(xpv1.Creating()),
					withProvisionPer(float32(50)),
					withNetworkType(&networkType),
					withState(v1alpha2.StateProvisioning),
				),
				observation: managed.ExternalObservation{
					ResourceExists:    true,
					ResourceUpToDate:  true,
					ConnectionDetails: managed.ConnectionDetails{},
				},
			},
		},
		"ObservedDeviceQueued": {
			client: &external{
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				client: &fake.MockClient{
					MockGet: func(deviceID string, getOpt *packngo.GetOptions) (*packngo.Device, *packngo.Response, error) {
						d := &packngo.Device{
							State:        v1alpha2.StateQueued,
							ProvisionPer: float32(50),
							AlwaysPXE:    *alwaysPXE,
						}

						return d, nil, nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  device(),
			},
			want: want{
				mg: device(
					withInitializerParams(initializerParams{}),
					withConditions(xpv1.Unavailable()),
					withProvisionPer(float32(50)),
					withNetworkType(&networkType),
					withState(v1alpha2.StateQueued)),
				observation: managed.ExternalObservation{
					ResourceExists:    true,
					ResourceUpToDate:  true,
					ConnectionDetails: managed.ConnectionDetails{},
				},
			},
		},
		"ObservedDeviceDoesNotExist": {
			client: &external{client: &fake.MockClient{
				MockGet: func(deviceID string, getOpt *packngo.GetOptions) (*packngo.Device, *packngo.Response, error) {
					return nil, nil, &packngo.ErrorResponse{
						Response: &http.Response{
							StatusCode: http.StatusNotFound,
						},
					}
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  device(),
			},
			want: want{
				mg:          device(),
				observation: managed.ExternalObservation{ResourceExists: false},
			},
		},
		"NotDevice": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotDevice),
			},
		},
		"FailedToGetDevice": {
			client: &external{client: &fake.MockClient{
				MockGet: func(deviceID string, getOpt *packngo.GetOptions) (*packngo.Device, *packngo.Response, error) {
					return nil, nil, errorBoom
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  device(),
			},
			want: want{
				mg:  device(),
				err: errors.Wrap(errorBoom, errGetDevice),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.client.Observe(tc.args.ctx, tc.args.mg)

			if diff := cmp.Diff(tc.want.observation, got, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Observe(): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Observe(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions(), packettest.EquateQuantities()); diff != "" {
				t.Errorf("resource.Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg       resource.Managed
		creation managed.ExternalCreation
		err      error
	}

	cases := map[string]struct {
		client managed.ExternalClient
		args   args
		want   want
	}{
		"CreatedInstance": {
			client: &external{
				client: &fake.MockClient{
					MockGetProjectID: projectIDFromCredentials,
					MockCreate: func(createRequest *packngo.DeviceCreateRequest) (*packngo.Device, *packngo.Response, error) {
						d := &packngo.Device{
							ID: deviceName,
						}

						return d, nil, nil
					},
				},
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  device(),
			},
			want: want{
				mg: device(
					withConditions(xpv1.Creating()),
					withID(deviceName),
				),
				creation: managed.ExternalCreation{
					ConnectionDetails: managed.ConnectionDetails{},
				},
			},
		},
		"NotDevice": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotDevice),
			},
		},
		"FailedToCreateDevice": {
			client: &external{client: &fake.MockClient{
				MockGetProjectID: projectIDFromCredentials,
				MockCreate: func(createRequest *packngo.DeviceCreateRequest) (*packngo.Device, *packngo.Response, error) {
					return nil, nil, errorBoom
				},
			}},

			args: args{
				ctx: context.Background(),
				mg:  device(),
			},
			want: want{
				mg:  device(withConditions(xpv1.Creating())),
				err: errors.Wrap(errorBoom, errCreateDevice),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.client.Create(tc.args.ctx, tc.args.mg)

			if diff := cmp.Diff(tc.want.creation, got, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Create(): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Create(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions(), packettest.EquateQuantities()); diff != "" {
				t.Errorf("resource.Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg     resource.Managed
		update managed.ExternalUpdate
		err    error
	}

	cases := map[string]struct {
		client managed.ExternalClient
		args   args
		want   want
	}{
		"NoUpdateNeeded": {
			client: &external{client: &fake.MockClient{
				MockUpdate: func(deviceID string, createRequest *packngo.DeviceUpdateRequest) (*packngo.Device, *packngo.Response, error) {
					return &packngo.Device{}, nil, nil
				},
				MockGet: func(deviceID string, getOpt *packngo.GetOptions) (*packngo.Device, *packngo.Response, error) {
					return &packngo.Device{}, nil, nil
				},
			}},
			args: args{
				ctx: context.Background(),
				mg:  device(),
			},
			want: want{
				mg: device(withConditions()),
			},
		},
		"UpdatedInstanceNetworkType": {
			client: &external{client: &fake.MockClient{
				MockGet: func(deviceID string, getOpt *packngo.GetOptions) (*packngo.Device, *packngo.Response, error) {
					d := &packngo.Device{}
					target := packngo.NetworkTypeHybrid
					d.Network = mockNetworkTypeConfigs[target].Network
					d.NetworkPorts = mockNetworkTypeConfigs[target].NetworkPorts

					return d, nil, nil
				},
				MockDeviceToNetworkType: func(deviceID string, networkType string) (*packngo.Device, error) {
					return nil, nil
				},
			}},
			args: args{
				ctx: context.Background(),
				mg:  device(withNetworkType(&networkType)),
			},
			want: want{
				mg: device(withNetworkType(&networkType), withConditions()),
			},
		},
		"UpdatedInstance": {
			client: &external{client: &fake.MockClient{
				MockUpdate: func(deviceID string, createRequest *packngo.DeviceUpdateRequest) (*packngo.Device, *packngo.Response, error) {
					return &packngo.Device{}, nil, nil
				},
				MockGet: func(deviceID string, getOpt *packngo.GetOptions) (*packngo.Device, *packngo.Response, error) {
					d := &packngo.Device{
						AlwaysPXE: false,
					}

					return d, nil, nil
				},
			}},
			args: args{
				ctx: context.Background(),
				mg:  device(),
			},
			want: want{
				mg: device(withConditions()),
			},
		},
		"NotCloudMemorystoreInstance": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotDevice),
			},
		},
		"FailedToUpdateInstance": {
			client: &external{client: &fake.MockClient{
				MockUpdate: func(deviceID string, createRequest *packngo.DeviceUpdateRequest) (*packngo.Device, *packngo.Response, error) {
					return nil, nil, errorBoom
				},
				MockGet: func(deviceID string, getOpt *packngo.GetOptions) (*packngo.Device, *packngo.Response, error) {
					return &packngo.Device{}, nil, nil
				},
			}},

			args: args{
				ctx: context.Background(),
				mg:  device(),
			},
			want: want{
				mg:  device(),
				err: errors.Wrap(errorBoom, errUpdateDevice),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.client.Update(tc.args.ctx, tc.args.mg)

			if diff := cmp.Diff(tc.want.update, got, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Update(): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Update(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions(), packettest.EquateQuantities()); diff != "" {
				t.Errorf("resource.Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg  resource.Managed
		err error
	}

	cases := map[string]struct {
		client managed.ExternalClient
		args   args
		want   want
	}{
		"DeletedInstance": {
			client: &external{client: &fake.MockClient{
				MockDelete: func(deviceID string, force bool) (*packngo.Response, error) {
					return nil, nil
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  device(),
			},
			want: want{
				mg: device(withConditions(xpv1.Deleting())),
			},
		},
		"NotDeviceInstance": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotDevice),
			},
		},
		"FailedToDeleteInstance": {
			client: &external{client: &fake.MockClient{
				MockDelete: func(deviceID string, force bool) (*packngo.Response, error) {
					return nil, errorBoom
				},
			}},

			args: args{
				ctx: context.Background(),
				mg:  device(),
			},
			want: want{
				mg:  device(withConditions(xpv1.Deleting())),
				err: errors.Wrap(errorBoom, errDeleteDevice),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.client.Delete(tc.args.ctx, tc.args.mg)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Delete(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions(), packettest.EquateQuantities()); diff != "" {
				t.Errorf("resource.Managed: -want, +got:\n%s", diff)
			}
		})
	}
}
