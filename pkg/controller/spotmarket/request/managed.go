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

package request

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/packethost/crossplane-provider-packet/apis/spotmarket/v1alpha1"
	packetv1alpha2 "github.com/packethost/crossplane-provider-packet/apis/v1alpha2"
	packetclient "github.com/packethost/crossplane-provider-packet/pkg/clients"
	spotmarketclient "github.com/packethost/crossplane-provider-packet/pkg/clients/spotmarket"

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
	errNewClient           = "cannot create new Request client"
	errNotRequest          = "managed resource is not a Request"
	errGetPort             = "cannot get Port"
	errCreateRequest       = "cannot create Request"
	errDeleteRequest       = "cannot delete Request"
	errManagedUpdateFailed = "cannot update Request custom resource"
)

// SetupRequest adds a controller that reconciles Requests
func SetupRequest(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.RequestGroupKind)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.RequestGroupVersionKind),
		managed.WithExternalConnecter(&connecter{kube: mgr.GetClient()}),
		managed.WithInitializers(),
		managed.WithConnectionPublishers(),
		managed.WithReferenceResolver(managed.NewAPISimpleReferenceResolver(mgr.GetClient())),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Request{}).
		Complete(r)
}

type connecter struct {
	kube        client.Client
	newClientFn func(ctx context.Context, credentials []byte, projectID string) (spotmarketclient.ClientWithDefaults, error)
}

func (c *connecter) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	g, ok := mg.(*v1alpha1.Request)
	if !ok {
		return nil, errors.New(errNotRequest)
	}

	p := &packetv1alpha2.Provider{}
	n := meta.NamespacedNameOf(g.Spec.ProviderReference)
	if err := c.kube.Get(ctx, n, p); err != nil {
		return nil, errors.Wrap(err, errGetProvider)
	}

	if p.GetCredentialsSecretReference() == nil {
		return nil, errors.New(errProviderSecretNil)
	}

	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: p.Spec.CredentialsSecretRef.Namespace, Name: p.Spec.CredentialsSecretRef.Name}
	if err := c.kube.Get(ctx, n, s); err != nil {
		return nil, errors.Wrap(err, errGetProviderSecret)
	}
	newClientFn := spotmarketclient.NewClient
	if c.newClientFn != nil {
		newClientFn = c.newClientFn
	}
	client, err := newClientFn(ctx, s.Data[p.Spec.CredentialsSecretRef.Key], p.Spec.ProjectID)

	return &external{kube: c.kube, client: client}, errors.Wrap(err, errNewClient)
}

type external struct {
	kube   client.Client
	client spotmarketclient.ClientWithDefaults
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	req, ok := mg.(*v1alpha1.Request)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotRequest)
	}

	// Observe spotmarket
	spotmarket, _, err := e.client.Get(meta.GetExternalName(req), nil)
	if packetclient.IsNotFound(err) {
		return managed.ExternalObservation{}, errors.New("spotmarket does not exist")
	}
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetPort)
	}

	o := managed.ExternalObservation{
		ResourceExists:   false,
		ResourceUpToDate: true,
	}

	for _, net := range spotmarket.AttachedVirtualNetworks {
		if strings.TrimPrefix(net.Href, "/spot-market-requests/") == req.Spec.ForProvider.ID {
			req.Status.SetConditions(runtimev1alpha1.Available())
			resource.SetBindable(req)
			o.ResourceExists = true
		}
	}

	meta.SetExternalName(req, spotmarket.ID)
	return o, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	req, ok := mg.(*v1alpha1.Request)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotRequest)
	}

	req.Status.SetConditions(runtimev1alpha1.Creating())

	create := spotmarketclient.CreateFromRequest(req, e.client.GetProjectID(packetclient.CredentialProjectID))
	request, _, err := e.client.Create(create)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateRequest)
	}

	req.Status.AtProvider.ID = request.ID
	meta.SetExternalName(req, request.ID)
	err = e.kube.Update(ctx, req)
	return managed.ExternalCreation{}, errors.Wrap(err, errManagedUpdateFailed)
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	// NOTE(displague): Request cannot be updated.
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	req, ok := mg.(*v1alpha1.Request)
	if !ok {
		return errors.New(errNotRequest)
	}
	req.SetConditions(runtimev1alpha1.Deleting())
	_, err := e.client.Delete(meta.GetExternalName(req), false)
	return errors.Wrap(resource.Ignore(packetclient.IsNotFound, err), errDeleteRequest)
}
