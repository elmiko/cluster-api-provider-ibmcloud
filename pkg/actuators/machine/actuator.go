/*
Copyright 2021.

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

package machine

import (
	"context"
	"fmt"

	ibmclient "github.com/openshift/cluster-api-provider-ibmcloud/pkg/actuators/client"
	machinev1 "github.com/openshift/machine-api-operator/pkg/apis/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rec "k8s.io/client-go/tools/record"
	klog "k8s.io/klog/v2"
	controllerRuntimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	scopeFailFmt      = "%s: failed to create scope for machine: %w"
	reconcilerFailFmt = "%s: reconciler failed to %s machine: %w"
	createEventAction = "Create"
	updateEventAction = "Update"
	deleteEventAction = "Delete"
	noEventAction     = ""
)

// Actuator performs machine reconciliation
type Actuator struct {
	client           controllerRuntimeClient.Client
	eventRecorder    rec.EventRecorder
	ibmClientBuilder ibmclient.IbmcloudClientBuilderFuncType
	// TODO: client Builder Func type for building ibmcloud client
}

// ActuatorParams holds parameter information for Actuator.
type ActuatorParams struct {
	Client           controllerRuntimeClient.Client
	EventRecorder    rec.EventRecorder
	IbmClientBuilder ibmclient.IbmcloudClientBuilderFuncType
	// TODO: client Builder
}

// NewActuator returns an actuator.
func NewActuator(params ActuatorParams) *Actuator {
	return &Actuator{
		client:           params.Client,
		eventRecorder:    params.EventRecorder,
		ibmClientBuilder: params.IbmClientBuilder,
	}
}

// Set corresponding event based on error. It also returns the original error
// for convenience, so callers can do "return handleMachineError(...)".
func (a *Actuator) handleMachineError(machine *machinev1.Machine, err error, eventAction string) error {
	klog.Errorf("%v error: %v", machine.GetName(), err)
	if eventAction != noEventAction {
		a.eventRecorder.Eventf(machine, corev1.EventTypeWarning, "Failed"+eventAction, "%v", err)
	}
	return err
}

// Create - creates a machine and is invoked by the machine controller.
func (a *Actuator) Create(ctx context.Context, machine *machinev1.Machine) error {
	klog.Infof("%s: Creating machine", machine.Name)
	scope, err := newMachineScope(machineScopeParams{
		Context:          ctx,
		client:           a.client,
		machine:          machine,
		ibmClientBuilder: a.ibmClientBuilder,
	})
	if err != nil {
		fmtErr := fmt.Errorf(scopeFailFmt, machine.GetName(), err)
		return a.handleMachineError(machine, fmtErr, createEventAction)
	}
	if err := newReconciler(scope).create(); err != nil {
		// Update machine and machine status in case it was modified
		scope.Close()
		fmtErr := fmt.Errorf(reconcilerFailFmt, machine.GetName(), createEventAction, err)
		return a.handleMachineError(machine, fmtErr, createEventAction)
	}
	a.eventRecorder.Eventf(machine, corev1.EventTypeNormal, createEventAction, "Created Machine %v", machine.Name)
	return scope.Close()
}

// Update - maintains synchronization between Machine resource and an existing machine instance
func (a *Actuator) Update(ctx context.Context, machine *machinev1.Machine) error {
	klog.Infof("%s: Updating machine", machine.Name)
	scope, err := newMachineScope(machineScopeParams{
		Context:          ctx,
		client:           a.client,
		machine:          machine,
		ibmClientBuilder: a.ibmClientBuilder,
	})
	if err != nil {
		fmtErr := fmt.Errorf(scopeFailFmt, machine.GetName(), err)
		return a.handleMachineError(machine, fmtErr, updateEventAction)
	}
	if err := newReconciler(scope).update(); err != nil {
		// Update machine and machine status in case it was modified
		scope.Close()
		fmtErr := fmt.Errorf(reconcilerFailFmt, machine.GetName(), updateEventAction, err)
		return a.handleMachineError(machine, fmtErr, updateEventAction)
	}
	a.eventRecorder.Eventf(machine, corev1.EventTypeNormal, updateEventAction, "Updated Machine %v", machine.Name)
	return scope.Close()
}

// Exists - checks if the machine exist.
// IMPORTANT: Exists() does not update Spec/Status obj, Only create() & update() stores the Spec and Status. Do not: scope.Close()
func (a *Actuator) Exists(ctx context.Context, machine *machinev1.Machine) (bool, error) {
	klog.Infof("%s: Checking if machine exists", machine.Name)
	scope, err := newMachineScope(machineScopeParams{
		Context:          ctx,
		client:           a.client,
		machine:          machine,
		ibmClientBuilder: a.ibmClientBuilder,
	})
	if err != nil {
		return false, fmt.Errorf(scopeFailFmt, machine.Name, err)
	}
	return newReconciler(scope).exists()
}

// Delete - deletes a machine
func (a *Actuator) Delete(ctx context.Context, machine *machinev1.Machine) error {
	klog.Infof("%s: Deleting machine", machine.Name)
	scope, err := newMachineScope(machineScopeParams{
		Context:          ctx,
		client:           a.client,
		machine:          machine,
		ibmClientBuilder: a.ibmClientBuilder,
	})
	if err != nil {
		fmtErr := fmt.Errorf(scopeFailFmt, machine.GetName(), err)
		return a.handleMachineError(machine, fmtErr, deleteEventAction)
	}
	if err := newReconciler(scope).delete(); err != nil {
		fmtErr := fmt.Errorf(reconcilerFailFmt, machine.GetName(), deleteEventAction, err)
		return a.handleMachineError(machine, fmtErr, deleteEventAction)
	}
	a.eventRecorder.Eventf(machine, corev1.EventTypeNormal, deleteEventAction, "Deleted machine %v", machine.Name)
	return nil
}
