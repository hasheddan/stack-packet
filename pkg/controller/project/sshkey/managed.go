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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/packethost/crossplane-provider-equinix-metal/apis/project/v1alpha1"
	packetv1beta1 "github.com/packethost/crossplane-provider-equinix-metal/apis/v1beta1"
	packetclient "github.com/packethost/crossplane-provider-equinix-metal/pkg/clients"
	sshkeyclient "github.com/packethost/crossplane-provider-equinix-metal/pkg/clients/sshkey"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errProviderSecretNil   = "cannot find Secret reference on Provider"
	errGetProvider         = "cannot get Provider"
	errGetProviderSecret   = "cannot get Provider Secret"
	errNewClient           = "cannot create new SSHKey client"
	errNotSSHKey           = "managed resource is not a SSHKey"
	errGetSSHKey           = "cannot get SSHKey"
	errCreateSSHKey        = "cannot create SSHKey"
	errUpdateSSHKey        = "cannot update SSHKey"
	errDeleteSSHKey        = "cannot delete SSHKey"
	errManagedUpdateFailed = "cannot update SSHKey custom resource"
)

// SetupSSHKey adds a controller that reconciles SSHKeys
func SetupSSHKey(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.SSHKeyGroupKind)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.SSHKeyGroupVersionKind),
		managed.WithExternalConnecter(&connecter{kube: mgr.GetClient()}),
		managed.WithInitializers(),
		managed.WithConnectionPublishers(),
		managed.WithReferenceResolver(managed.NewAPISimpleReferenceResolver(mgr.GetClient())),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.SSHKey{}).
		Complete(r)
}

type connecter struct {
	kube        client.Client
	newClientFn func(ctx context.Context, credentials []byte, projectID string) (sshkeyclient.ClientWithDefaults, error)
}

func (c *connecter) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	g, ok := mg.(*v1alpha1.SSHKey)
	if !ok {
		return nil, errors.New(errNotSSHKey)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	p := &packetv1beta1.Provider{}
	if err := c.kube.Get(ctx, g.Spec.ProviderConfigReference.Name, p); err != nil {
		return nil, errors.Wrap(err, errGetProvider)
	}

	ref := p.Spec.Credentials.SecretRef
	if ref == nil {
		return nil, errors.New(errProviderSecretNil)
	}

	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
	if err := c.kube.Get(ctx, n, s); err != nil {
		return nil, errors.Wrap(err, errGetProviderSecret)
	}
	newClientFn := sshkeyclient.NewClient
	if c.newClientFn != nil {
		newClientFn = c.newClientFn
	}
	client, err := newClientFn(ctx, s.Data[ref.Key], p.Spec.ProjectID)

	return &external{kube: c.kube, client: client}, errors.Wrap(err, errNewClient)
}

type external struct {
	kube   client.Client
	client sshkeyclient.ClientWithDefaults
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	key, ok := mg.(*v1alpha1.SSHKey)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotSSHKey)
	}

	// Observe sshkey
	sshKey, _, err := e.client.Get(meta.GetExternalName(key), nil)
	if packetclient.IsNotFound(err) {
		return managed.ExternalObservation{}, errors.New("sshKey does not exist")
	}
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetSSHKey)
	}

	o := managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: sshkeyclient.IsUpToDate(key, sshKey),
	}

	meta.SetExternalName(key, sshKey.ID)
	return o, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	req, ok := mg.(*v1alpha1.SSHKey)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotSSHKey)
	}

	req.Status.SetConditions(runtimev1alpha1.Creating())

	create := sshkeyclient.CreateFromSSHKey(req, e.client.GetProjectID(packetclient.CredentialProjectID))
	sshKey, _, err := e.client.Create(create)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateSSHKey)
	}

	req.Status.AtProvider.ID = sshKey.ID
	meta.SetExternalName(req, sshKey.ID)
	err = e.kube.Update(ctx, req)
	return managed.ExternalCreation{}, errors.Wrap(err, errManagedUpdateFailed)
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	s, ok := mg.(*v1alpha1.SSHKey)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotSSHKey)
	}
	_, _, err := e.client.Update(meta.GetExternalName(s), sshkeyclient.NewUpdateSSHKeyRequest(s))
	return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateSSHKey)
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	req, ok := mg.(*v1alpha1.SSHKey)
	if !ok {
		return errors.New(errNotSSHKey)
	}
	req.SetConditions(runtimev1alpha1.Deleting())
	_, err := e.client.Delete(meta.GetExternalName(req), false)
	return errors.Wrap(resource.Ignore(packetclient.IsNotFound, err), errDeleteSSHKey)
}
