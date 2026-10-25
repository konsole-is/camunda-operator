/*
Copyright 2026.

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

package camundamanagementcluster

import (
	"fmt"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/serviceaccount"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/accounthash"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// ServiceAccountReference is an account that a running part of the
// management plane names with create false. The operator does not create it,
// so it must exist.
type ServiceAccountReference struct {
	// Field is the spec block that names the account, such as
	// spec.serviceAccount.
	Field string
	// Name is the name of the account.
	Name string
}

// The suffixes of the derived account names: one for the account of the
// plane, and one for the account of each component block.
const (
	planeAccountSuffix      = "management"
	keycloakAccountSuffix   = "keycloak"
	identityAccountSuffix   = "identity"
	consoleAccountSuffix    = "console"
	webModelerAccountSuffix = "web-modeler"
)

// account is one ServiceAccount of the management plane.
type account struct {
	field     string
	component string
	spec      *v1.ServiceAccountSpec
	name      string
	// used reports whether pods run under the account: always for the account
	// of the plane, and for a component account while the spec runs the
	// component and sets its block.
	used bool
}

// serviceAccountsComponents renders the ServiceAccounts of the management
// plane, ahead of every workload, so that each account exists before a pod
// names it. With create false the operator neither applies nor deletes an
// account, and it removes its owner reference from an account that it
// created before, so the account outlives the plane. An account that no pod
// uses any more is deleted only when the plane owns it.
//
// The component carries no Suspend. An ocf ServiceAccount models no
// suspension, and the accounts stay while the workloads are scaled to zero.
func serviceAccountsComponents(in Input) (Built, error) {
	all := accounts(in.Cluster)

	usedNames := map[string]bool{}
	for _, a := range all {
		if a.used {
			usedNames[a.name] = true
		}
	}

	builder := component.NewComponentBuilder().
		WithName(ComponentServiceAccounts).
		WithConditionType(component.ConditionType(v1.ConditionServiceAccountsReady))
	for _, a := range all {
		// The release of an unused account removes the owner reference that
		// the account of the same name carries for the pods that use it.
		if !a.used && usedNames[a.name] {
			continue
		}

		resource, err := serviceaccount.NewBuilder(&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:        a.name,
				Namespace:   in.Cluster.Namespace,
				Labels:      managedLabels(in, a.component),
				Annotations: accountAnnotations(a.spec),
			},
		}).Build()
		if err != nil {
			return Built{}, fmt.Errorf("building the %s component: %w", ComponentServiceAccounts, err)
		}
		builder = builder.WithResource(resource, accountOptions(a)...)
	}

	comp, err := builder.Build()
	if err != nil {
		return Built{}, fmt.Errorf("building the %s component: %w", ComponentServiceAccounts, err)
	}

	comps := []*component.Component{comp}

	return Built{Components: comps, Ready: comps}, nil
}

// accountOptions registers account a. With create false, the account is the
// user's.
func accountOptions(a account) []component.ResourceOption {
	if !a.spec.Creates() {
		return []component.ResourceOption{component.OrphanWhen(true)}
	}

	return []component.ResourceOption{component.GatedBy(feature.NewBooleanGate(a.used)), component.DeleteOnlyIfOwned()}
}

// accounts returns every account of mc: the account of the plane first, then
// the account of each component block, also of a component that does not run.
// The name of an unset block is its derived name, so that the account it left
// behind is deleted.
func accounts(mc *v1.CamundaManagementCluster) []account {
	spec := &mc.Spec

	var keycloak, console, webModeler *v1.ServiceAccountSpec
	if spec.IdentityProvider.Keycloak != nil {
		keycloak = spec.IdentityProvider.Keycloak.ServiceAccount
	}
	if spec.Console != nil {
		console = spec.Console.ServiceAccount
	}
	if spec.WebModeler != nil {
		webModeler = spec.WebModeler.ServiceAccount
	}

	return []account{
		{
			field:     "spec.serviceAccount",
			component: ComponentServiceAccounts,
			spec:      spec.ServiceAccount,
			name:      accountName(mc, spec.ServiceAccount, planeAccountSuffix),
			used:      true,
		},
		componentAccount(mc, "spec.identityProvider.keycloak.serviceAccount", ComponentKeycloak, keycloak),
		componentAccount(mc, "spec.identity.serviceAccount", ComponentIdentity, spec.Identity.ServiceAccount),
		componentAccount(mc, "spec.console.serviceAccount", ComponentConsole, console),
		componentAccount(mc, "spec.webModeler.serviceAccount", ComponentWebModeler, webModeler),
	}
}

// componentAccount returns the account of the block spec of comp. A block is
// set only while the spec runs its component, so a set block is used.
func componentAccount(mc *v1.CamundaManagementCluster, field, comp string, spec *v1.ServiceAccountSpec) account {
	return account{
		field:     field,
		component: comp,
		spec:      spec,
		name:      accountName(mc, spec, componentAccountSuffixes[comp]),
		used:      spec != nil,
	}
}

// componentAccountSuffixes holds the suffix of the derived account name of
// each component that has a block of its own.
var componentAccountSuffixes = map[string]string{
	ComponentKeycloak:   keycloakAccountSuffix,
	ComponentIdentity:   identityAccountSuffix,
	ComponentConsole:    consoleAccountSuffix,
	ComponentWebModeler: webModelerAccountSuffix,
}

// accountName returns the name that spec sets, or the name of mc with the
// suffix. A ServiceAccount name is a DNS subdomain, the bound of the derived
// name of a CamundaCluster account too.
func accountName(mc *v1.CamundaManagementCluster, spec *v1.ServiceAccountSpec, suffix string) string {
	if spec != nil && spec.Name != "" {
		return spec.Name
	}

	return labels.BoundedName(mc.Name, validation.DNS1123SubdomainMaxLength-len(suffix)-1) + "-" + suffix
}

// accountAnnotations returns the annotations of spec, or nil when there are
// none.
func accountAnnotations(spec *v1.ServiceAccountSpec) map[string]string {
	if spec == nil || len(spec.Annotations) == 0 {
		return nil
	}

	return spec.Annotations
}

// ServiceAccountName returns the account of spec.serviceAccount of mc: the
// name it sets, or <name>-management. Every workload without an account of
// its own runs under it, and every PluginRequest of the plane targets it.
func ServiceAccountName(mc *v1.CamundaManagementCluster) string {
	return accounts(mc)[0].name
}

// PodServiceAccountName returns the account that the pods of comp run under:
// the account of the serviceAccount block of its component, else
// ServiceAccountName. comp is ComponentKeycloak, ComponentIdentity,
// ComponentConsole, ComponentWebModelerRestapi, or
// ComponentWebModelerWebsockets.
func PodServiceAccountName(mc *v1.CamundaManagementCluster, comp string) string {
	return podAccount(mc, comp).name
}

// podAccount returns the account that the pods of comp run under.
func podAccount(mc *v1.CamundaManagementCluster, comp string) account {
	if comp == ComponentWebModelerRestapi || comp == ComponentWebModelerWebsockets {
		comp = ComponentWebModeler
	}

	all := accounts(mc)
	for _, a := range all[1:] {
		if a.component == comp && a.used {
			return a
		}
	}

	return all[0]
}

// ReferencedServiceAccounts returns each account that a running part of mc
// names with create false, the account of the plane first.
func ReferencedServiceAccounts(mc *v1.CamundaManagementCluster) []ServiceAccountReference {
	var refs []ServiceAccountReference
	for _, a := range accounts(mc) {
		if a.used && !a.spec.Creates() {
			refs = append(refs, ServiceAccountReference{Field: a.field, Name: a.name})
		}
	}

	return refs
}

// podAccountMetadata returns the labels and the annotations that the pods of
// comp need for their account. A hash of the annotations that the operator
// renders on the account rolls the pods when they change, because workload
// identity reads them when a pod starts. Azure binds a pod to the client ID
// of its account only while the pod carries the use label.
func podAccountMetadata(mc *v1.CamundaManagementCluster, comp string) (podLabels, podAnnotations map[string]string) {
	a := podAccount(mc, comp)
	annotations := accountAnnotations(a.spec)

	if _, ok := annotations[v1.AzureClientIDAnnotation]; ok {
		podLabels = map[string]string{v1.AzureWorkloadIdentityUseLabel: "true"}
	}
	if hash := accounthash.Of(annotations); hash != "" && a.spec.Creates() {
		podAnnotations = map[string]string{accounthash.Annotation: hash}
	}

	return podLabels, podAnnotations
}
