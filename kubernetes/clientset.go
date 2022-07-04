/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package kubernetes

import (
	discovery "k8s.io/client-go/discovery"
	admissionv1 "k8s.io/client-go/kubernetes/typed/admission/v1"
	admissionv1beta1 "k8s.io/client-go/kubernetes/typed/admission/v1beta1"
	admissionregistrationv1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1beta1"
	internalv1alpha1 "k8s.io/client-go/kubernetes/typed/apiserverinternal/v1alpha1"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	appsv1beta1 "k8s.io/client-go/kubernetes/typed/apps/v1beta1"
	appsv1beta2 "k8s.io/client-go/kubernetes/typed/apps/v1beta2"
	authenticationv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	authenticationv1beta1 "k8s.io/client-go/kubernetes/typed/authentication/v1beta1"
	authorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	authorizationv1beta1 "k8s.io/client-go/kubernetes/typed/authorization/v1beta1"
	autoscalingv1 "k8s.io/client-go/kubernetes/typed/autoscaling/v1"
	autoscalingv2 "k8s.io/client-go/kubernetes/typed/autoscaling/v2"
	autoscalingv2beta1 "k8s.io/client-go/kubernetes/typed/autoscaling/v2beta1"
	autoscalingv2beta2 "k8s.io/client-go/kubernetes/typed/autoscaling/v2beta2"
	batchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	batchv1beta1 "k8s.io/client-go/kubernetes/typed/batch/v1beta1"
	certificatesv1 "k8s.io/client-go/kubernetes/typed/certificates/v1"
	certificatesv1beta1 "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	coordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	coordinationv1beta1 "k8s.io/client-go/kubernetes/typed/coordination/v1beta1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	discoveryv1 "k8s.io/client-go/kubernetes/typed/discovery/v1"
	discoveryv1beta1 "k8s.io/client-go/kubernetes/typed/discovery/v1beta1"
	eventsv1 "k8s.io/client-go/kubernetes/typed/events/v1"
	eventsv1beta1 "k8s.io/client-go/kubernetes/typed/events/v1beta1"
	extensionsv1beta1 "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	flowcontrolv1alpha1 "k8s.io/client-go/kubernetes/typed/flowcontrol/v1alpha1"
	flowcontrolv1beta1 "k8s.io/client-go/kubernetes/typed/flowcontrol/v1beta1"
	flowcontrolv1beta2 "k8s.io/client-go/kubernetes/typed/flowcontrol/v1beta2"
	imagepolicyv1alpha1 "k8s.io/client-go/kubernetes/typed/imagepolicy/v1alpha1"
	networkingv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
	networkingv1beta1 "k8s.io/client-go/kubernetes/typed/networking/v1beta1"
	nodev1 "k8s.io/client-go/kubernetes/typed/node/v1"
	nodev1alpha1 "k8s.io/client-go/kubernetes/typed/node/v1alpha1"
	nodev1beta1 "k8s.io/client-go/kubernetes/typed/node/v1beta1"
	policyv1 "k8s.io/client-go/kubernetes/typed/policy/v1"
	policyv1beta1 "k8s.io/client-go/kubernetes/typed/policy/v1beta1"
	rbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	rbacv1alpha1 "k8s.io/client-go/kubernetes/typed/rbac/v1alpha1"
	rbacv1beta1 "k8s.io/client-go/kubernetes/typed/rbac/v1beta1"
	schedulingv1 "k8s.io/client-go/kubernetes/typed/scheduling/v1"
	schedulingv1alpha1 "k8s.io/client-go/kubernetes/typed/scheduling/v1alpha1"
	schedulingv1beta1 "k8s.io/client-go/kubernetes/typed/scheduling/v1beta1"
	storagev1 "k8s.io/client-go/kubernetes/typed/storage/v1"
	storagev1alpha1 "k8s.io/client-go/kubernetes/typed/storage/v1alpha1"
	storagev1beta1 "k8s.io/client-go/kubernetes/typed/storage/v1beta1"
	rest "k8s.io/client-go/rest"
)

type Interface interface {
	Discovery() discovery.DiscoveryInterface
	AdmissionV1() admissionv1.AdmissionV1Interface
	AdmissionV1beta1() admissionv1beta1.AdmissionV1beta1Interface
	AdmissionregistrationV1() admissionregistrationv1.AdmissionregistrationV1Interface
	AdmissionregistrationV1beta1() admissionregistrationv1beta1.AdmissionregistrationV1beta1Interface
	InternalV1alpha1() internalv1alpha1.InternalV1alpha1Interface
	AppsV1() appsv1.AppsV1Interface
	AppsV1beta1() appsv1beta1.AppsV1beta1Interface
	AppsV1beta2() appsv1beta2.AppsV1beta2Interface
	AuthenticationV1() authenticationv1.AuthenticationV1Interface
	AuthenticationV1beta1() authenticationv1beta1.AuthenticationV1beta1Interface
	AuthorizationV1() authorizationv1.AuthorizationV1Interface
	AuthorizationV1beta1() authorizationv1beta1.AuthorizationV1beta1Interface
	AutoscalingV1() autoscalingv1.AutoscalingV1Interface
	AutoscalingV2() autoscalingv2.AutoscalingV2Interface
	AutoscalingV2beta1() autoscalingv2beta1.AutoscalingV2beta1Interface
	AutoscalingV2beta2() autoscalingv2beta2.AutoscalingV2beta2Interface
	BatchV1() batchv1.BatchV1Interface
	BatchV1beta1() batchv1beta1.BatchV1beta1Interface
	CertificatesV1() certificatesv1.CertificatesV1Interface
	CertificatesV1beta1() certificatesv1beta1.CertificatesV1beta1Interface
	CoordinationV1() coordinationv1.CoordinationV1Interface
	CoordinationV1beta1() coordinationv1beta1.CoordinationV1beta1Interface
	CoreV1() corev1.CoreV1Interface
	DiscoveryV1() discoveryv1.DiscoveryV1Interface
	DiscoveryV1beta1() discoveryv1beta1.DiscoveryV1beta1Interface
	EventsV1() eventsv1.EventsV1Interface
	EventsV1beta1() eventsv1beta1.EventsV1beta1Interface
	ExtensionsV1beta1() extensionsv1beta1.ExtensionsV1beta1Interface
	FlowcontrolV1alpha1() flowcontrolv1alpha1.FlowcontrolV1alpha1Interface
	FlowcontrolV1beta1() flowcontrolv1beta1.FlowcontrolV1beta1Interface
	FlowcontrolV1beta2() flowcontrolv1beta2.FlowcontrolV1beta2Interface
	ImagepolicyV1alpha1() imagepolicyv1alpha1.ImagepolicyV1alpha1Interface
	NetworkingV1() networkingv1.NetworkingV1Interface
	NetworkingV1beta1() networkingv1beta1.NetworkingV1beta1Interface
	NodeV1() nodev1.NodeV1Interface
	NodeV1alpha1() nodev1alpha1.NodeV1alpha1Interface
	NodeV1beta1() nodev1beta1.NodeV1beta1Interface
	PolicyV1() policyv1.PolicyV1Interface
	PolicyV1beta1() policyv1beta1.PolicyV1beta1Interface
	RbacV1() rbacv1.RbacV1Interface
	RbacV1alpha1() rbacv1alpha1.RbacV1alpha1Interface
	RbacV1beta1() rbacv1beta1.RbacV1beta1Interface
	SchedulingV1() schedulingv1.SchedulingV1Interface
	SchedulingV1alpha1() schedulingv1alpha1.SchedulingV1alpha1Interface
	SchedulingV1beta1() schedulingv1beta1.SchedulingV1beta1Interface
	StorageV1() storagev1.StorageV1Interface
	StorageV1alpha1() storagev1alpha1.StorageV1alpha1Interface
	StorageV1beta1() storagev1beta1.StorageV1beta1Interface
}

// Clientset contains the clients for groups. Each group has exactly one
// version included in a Clientset.
type Clientset struct {
	*discovery.DiscoveryClient
	client rest.Interface
}

// AdmissionV1 retrieves the AdmissionV1Client
func (c *Clientset) AdmissionV1() admissionv1.AdmissionV1Interface {
	return admissionv1.New(c.client)
}

// AdmissionV1beta1 retrieves the AdmissionV1beta1Client
func (c *Clientset) AdmissionV1beta1() admissionv1beta1.AdmissionV1beta1Interface {
	return admissionv1beta1.New(c.client)
}

// AdmissionregistrationV1 retrieves the AdmissionregistrationV1Client
func (c *Clientset) AdmissionregistrationV1() admissionregistrationv1.AdmissionregistrationV1Interface {
	return admissionregistrationv1.New(c.client)
}

// AdmissionregistrationV1beta1 retrieves the AdmissionregistrationV1beta1Client
func (c *Clientset) AdmissionregistrationV1beta1() admissionregistrationv1beta1.AdmissionregistrationV1beta1Interface {
	return admissionregistrationv1beta1.New(c.client)
}

// InternalV1alpha1 retrieves the InternalV1alpha1Client
func (c *Clientset) InternalV1alpha1() internalv1alpha1.InternalV1alpha1Interface {
	return internalv1alpha1.New(c.client)
}

// AppsV1 retrieves the AppsV1Client
func (c *Clientset) AppsV1() appsv1.AppsV1Interface {
	return appsv1.New(c.client)
}

// AppsV1beta1 retrieves the AppsV1beta1Client
func (c *Clientset) AppsV1beta1() appsv1beta1.AppsV1beta1Interface {
	return appsv1beta1.New(c.client)
}

// AppsV1beta2 retrieves the AppsV1beta2Client
func (c *Clientset) AppsV1beta2() appsv1beta2.AppsV1beta2Interface {
	return appsv1beta2.New(c.client)
}

// AuthenticationV1 retrieves the AuthenticationV1Client
func (c *Clientset) AuthenticationV1() authenticationv1.AuthenticationV1Interface {
	return authenticationv1.New(c.client)
}

// AuthenticationV1beta1 retrieves the AuthenticationV1beta1Client
func (c *Clientset) AuthenticationV1beta1() authenticationv1beta1.AuthenticationV1beta1Interface {
	return authenticationv1beta1.New(c.client)
}

// AuthorizationV1 retrieves the AuthorizationV1Client
func (c *Clientset) AuthorizationV1() authorizationv1.AuthorizationV1Interface {
	return authorizationv1.New(c.client)
}

// AuthorizationV1beta1 retrieves the AuthorizationV1beta1Client
func (c *Clientset) AuthorizationV1beta1() authorizationv1beta1.AuthorizationV1beta1Interface {
	return authorizationv1beta1.New(c.client)
}

// AutoscalingV1 retrieves the AutoscalingV1Client
func (c *Clientset) AutoscalingV1() autoscalingv1.AutoscalingV1Interface {
	return autoscalingv1.New(c.client)
}

// AutoscalingV2 retrieves the AutoscalingV2Client
func (c *Clientset) AutoscalingV2() autoscalingv2.AutoscalingV2Interface {
	return autoscalingv2.New(c.client)
}

// AutoscalingV2beta1 retrieves the AutoscalingV2beta1Client
func (c *Clientset) AutoscalingV2beta1() autoscalingv2beta1.AutoscalingV2beta1Interface {
	return autoscalingv2beta1.New(c.client)
}

// AutoscalingV2beta2 retrieves the AutoscalingV2beta2Client
func (c *Clientset) AutoscalingV2beta2() autoscalingv2beta2.AutoscalingV2beta2Interface {
	return autoscalingv2beta2.New(c.client)
}

// BatchV1 retrieves the BatchV1Client
func (c *Clientset) BatchV1() batchv1.BatchV1Interface {
	return batchv1.New(c.client)
}

// BatchV1beta1 retrieves the BatchV1beta1Client
func (c *Clientset) BatchV1beta1() batchv1beta1.BatchV1beta1Interface {
	return batchv1beta1.New(c.client)
}

// CertificatesV1 retrieves the CertificatesV1Client
func (c *Clientset) CertificatesV1() certificatesv1.CertificatesV1Interface {
	return certificatesv1.New(c.client)
}

// CertificatesV1beta1 retrieves the CertificatesV1beta1Client
func (c *Clientset) CertificatesV1beta1() certificatesv1beta1.CertificatesV1beta1Interface {
	return certificatesv1beta1.New(c.client)
}

// CoordinationV1 retrieves the CoordinationV1Client
func (c *Clientset) CoordinationV1() coordinationv1.CoordinationV1Interface {
	return coordinationv1.New(c.client)
}

// CoordinationV1beta1 retrieves the CoordinationV1beta1Client
func (c *Clientset) CoordinationV1beta1() coordinationv1beta1.CoordinationV1beta1Interface {
	return coordinationv1beta1.New(c.client)
}

// CoreV1 retrieves the CoreV1Client
func (c *Clientset) CoreV1() corev1.CoreV1Interface {
	return corev1.New(c.client)
}

// DiscoveryV1 retrieves the DiscoveryV1Client
func (c *Clientset) DiscoveryV1() discoveryv1.DiscoveryV1Interface {
	return discoveryv1.New(c.client)
}

// DiscoveryV1beta1 retrieves the DiscoveryV1beta1Client
func (c *Clientset) DiscoveryV1beta1() discoveryv1beta1.DiscoveryV1beta1Interface {
	return discoveryv1beta1.New(c.client)
}

// EventsV1 retrieves the EventsV1Client
func (c *Clientset) EventsV1() eventsv1.EventsV1Interface {
	return eventsv1.New(c.client)
}

// EventsV1beta1 retrieves the EventsV1beta1Client
func (c *Clientset) EventsV1beta1() eventsv1beta1.EventsV1beta1Interface {
	return eventsv1beta1.New(c.client)
}

// ExtensionsV1beta1 retrieves the ExtensionsV1beta1Client
func (c *Clientset) ExtensionsV1beta1() extensionsv1beta1.ExtensionsV1beta1Interface {
	return extensionsv1beta1.New(c.client)
}

// FlowcontrolV1alpha1 retrieves the FlowcontrolV1alpha1Client
func (c *Clientset) FlowcontrolV1alpha1() flowcontrolv1alpha1.FlowcontrolV1alpha1Interface {
	return flowcontrolv1alpha1.New(c.client)
}

// FlowcontrolV1beta1 retrieves the FlowcontrolV1beta1Client
func (c *Clientset) FlowcontrolV1beta1() flowcontrolv1beta1.FlowcontrolV1beta1Interface {
	return flowcontrolv1beta1.New(c.client)
}

// FlowcontrolV1beta2 retrieves the FlowcontrolV1beta2Client
func (c *Clientset) FlowcontrolV1beta2() flowcontrolv1beta2.FlowcontrolV1beta2Interface {
	return flowcontrolv1beta2.New(c.client)
}

// ImagepolicyV1alpha1 retrieves the ImagepolicyV1alpha1Client
func (c *Clientset) ImagepolicyV1alpha1() imagepolicyv1alpha1.ImagepolicyV1alpha1Interface {
	return imagepolicyv1alpha1.New(c.client)
}

// NetworkingV1 retrieves the NetworkingV1Client
func (c *Clientset) NetworkingV1() networkingv1.NetworkingV1Interface {
	return networkingv1.New(c.client)
}

// NetworkingV1beta1 retrieves the NetworkingV1beta1Client
func (c *Clientset) NetworkingV1beta1() networkingv1beta1.NetworkingV1beta1Interface {
	return networkingv1beta1.New(c.client)
}

// NodeV1 retrieves the NodeV1Client
func (c *Clientset) NodeV1() nodev1.NodeV1Interface {
	return nodev1.New(c.client)
}

// NodeV1alpha1 retrieves the NodeV1alpha1Client
func (c *Clientset) NodeV1alpha1() nodev1alpha1.NodeV1alpha1Interface {
	return nodev1alpha1.New(c.client)
}

// NodeV1beta1 retrieves the NodeV1beta1Client
func (c *Clientset) NodeV1beta1() nodev1beta1.NodeV1beta1Interface {
	return nodev1beta1.New(c.client)
}

// PolicyV1 retrieves the PolicyV1Client
func (c *Clientset) PolicyV1() policyv1.PolicyV1Interface {
	return policyv1.New(c.client)
}

// PolicyV1beta1 retrieves the PolicyV1beta1Client
func (c *Clientset) PolicyV1beta1() policyv1beta1.PolicyV1beta1Interface {
	return policyv1beta1.New(c.client)
}

// RbacV1 retrieves the RbacV1Client
func (c *Clientset) RbacV1() rbacv1.RbacV1Interface {
	return rbacv1.New(c.client)
}

// RbacV1alpha1 retrieves the RbacV1alpha1Client
func (c *Clientset) RbacV1alpha1() rbacv1alpha1.RbacV1alpha1Interface {
	return rbacv1alpha1.New(c.client)
}

// RbacV1beta1 retrieves the RbacV1beta1Client
func (c *Clientset) RbacV1beta1() rbacv1beta1.RbacV1beta1Interface {
	return rbacv1beta1.New(c.client)
}

// SchedulingV1 retrieves the SchedulingV1Client
func (c *Clientset) SchedulingV1() schedulingv1.SchedulingV1Interface {
	return schedulingv1.New(c.client)
}

// SchedulingV1alpha1 retrieves the SchedulingV1alpha1Client
func (c *Clientset) SchedulingV1alpha1() schedulingv1alpha1.SchedulingV1alpha1Interface {
	return schedulingv1alpha1.New(c.client)
}

// SchedulingV1beta1 retrieves the SchedulingV1beta1Client
func (c *Clientset) SchedulingV1beta1() schedulingv1beta1.SchedulingV1beta1Interface {
	return schedulingv1beta1.New(c.client)
}

// StorageV1 retrieves the StorageV1Client
func (c *Clientset) StorageV1() storagev1.StorageV1Interface {
	return storagev1.New(c.client)
}

// StorageV1alpha1 retrieves the StorageV1alpha1Client
func (c *Clientset) StorageV1alpha1() storagev1alpha1.StorageV1alpha1Interface {
	return storagev1alpha1.New(c.client)
}

// StorageV1beta1 retrieves the StorageV1beta1Client
func (c *Clientset) StorageV1beta1() storagev1beta1.StorageV1beta1Interface {
	return storagev1beta1.New(c.client)
}

// Discovery retrieves the DiscoveryClient
func (c *Clientset) Discovery() discovery.DiscoveryInterface {
	if c == nil {
		return nil
	}
	return c.DiscoveryClient
}

// New creates a new Clientset for the given RESTClient.
func New(c rest.Interface) *Clientset {
	return &Clientset{
		client:          c,
		DiscoveryClient: discovery.NewDiscoveryClient(c),
	}
}
