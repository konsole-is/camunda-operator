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

package echo

import (
	"context"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// controllerName is the controller name that the registration publishes.
const controllerName = "conformance-echo"

// parametersSchema is the schema of the parameters of a request.
const parametersSchema = `{"type":"object","properties":{"message":{"type":"string"}}}`

// publish applies the registration, and tries again each second until the
// apply passes or ctx ends.
func (c *Controller) publish(ctx context.Context) error {
	err := wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		if err := c.apply(ctx, c.registration()); err != nil {
			log.FromContext(ctx).Error(err, "Could not apply the PluginRegistration", "name", c.opts.Registration)
			return false, nil
		}
		return true, nil
	})
	if ctx.Err() != nil {
		return nil
	}

	return err
}

func (c *Controller) registration() *pluginsv1alpha1.PluginRegistration {
	outputs := make([]pluginsv1alpha1.OutputDecl, 0, len(c.opts.Outputs))
	for _, o := range c.opts.Outputs {
		outputs = append(outputs, pluginsv1alpha1.OutputDecl{Key: o.Key})
	}

	return &pluginsv1alpha1.PluginRegistration{
		TypeMeta: metav1.TypeMeta{APIVersion: pluginsv1alpha1.GroupVersion.String(), Kind: "PluginRegistration"},
		ObjectMeta: metav1.ObjectMeta{
			Name: c.opts.Registration,
		},
		Spec: pluginsv1alpha1.PluginRegistrationSpec{
			Sections:       c.opts.Sections,
			Label:          "Echo",
			Mode:           pluginsv1alpha1.ModeOptional,
			Controller:     pluginsv1alpha1.ControllerInfo{Name: controllerName},
			ServiceAccount: c.opts.ServiceAccount,
			Parameters:     &apiextensionsv1.JSON{Raw: []byte(parametersSchema)},
			Inputs:         c.opts.Inputs,
			Outputs:        outputs,
			DeletionPolicies: []pluginsv1alpha1.DeletionPolicy{
				pluginsv1alpha1.DeletionPolicyDelete,
				pluginsv1alpha1.DeletionPolicyOrphan,
			},
		},
	}
}

// reconcileRegistration sets Ready=True on the registration, on the copy
// that it read, so that it keeps every condition of another writer.
func (c *Controller) reconcileRegistration(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var reg pluginsv1alpha1.PluginRegistration
	if err := c.client.Get(ctx, req.NamespacedName, &reg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	changed := meta.SetStatusCondition(&reg.Status.Conditions, metav1.Condition{
		Type:               v1.ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             v1.ReasonHealthy,
		Message:            "The echo controller serves the registration",
		ObservedGeneration: reg.Generation,
	})
	if !changed && reg.Status.ObservedGeneration == reg.Generation {
		return ctrl.Result{}, nil
	}

	reg.Status.ObservedGeneration = reg.Generation

	return requeueOnConflict(c.client.Status().Update(ctx, &reg))
}

// requeueOnConflict turns a conflict into a requeue with no error, because
// other writers share the object.
func requeueOnConflict(err error) (ctrl.Result, error) {
	if apierrors.IsConflict(err) {
		return ctrl.Result{RequeueAfter: conflictRequeue}, nil
	}

	return ctrl.Result{}, err
}

// conflictRequeue is the wait before the next pass after a conflict.
const conflictRequeue = 250 * time.Millisecond
