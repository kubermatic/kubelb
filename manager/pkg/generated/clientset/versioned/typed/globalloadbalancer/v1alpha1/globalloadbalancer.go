/*
Copyright 2020 Kubermatic GmbH.

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

package v1alpha1

import (
	v1alpha1 "k8c.io/kubelb/manager/pkg/api/globalloadbalancer/v1alpha1"
	scheme "k8c.io/kubelb/manager/pkg/generated/clientset/versioned/scheme"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// GlobalLoadBalancersGetter has a method to return a GlobalLoadBalancerInterface.
// A group's client should implement this interface.
type GlobalLoadBalancersGetter interface {
	GlobalLoadBalancers(namespace string) GlobalLoadBalancerInterface
}

// GlobalLoadBalancerInterface has methods to work with GlobalLoadBalancer resources.
type GlobalLoadBalancerInterface interface {
	Create(*v1alpha1.GlobalLoadBalancer) (*v1alpha1.GlobalLoadBalancer, error)
	Update(*v1alpha1.GlobalLoadBalancer) (*v1alpha1.GlobalLoadBalancer, error)
	UpdateStatus(*v1alpha1.GlobalLoadBalancer) (*v1alpha1.GlobalLoadBalancer, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.GlobalLoadBalancer, error)
	List(opts v1.ListOptions) (*v1alpha1.GlobalLoadBalancerList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.GlobalLoadBalancer, err error)
	GlobalLoadBalancerExpansion
}

// globalLoadBalancers implements GlobalLoadBalancerInterface
type globalLoadBalancers struct {
	client rest.Interface
	ns     string
}

// newGlobalLoadBalancers returns a GlobalLoadBalancers
func newGlobalLoadBalancers(c *KubelbV1alpha1Client, namespace string) *globalLoadBalancers {
	return &globalLoadBalancers{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the globalLoadBalancer, and returns the corresponding globalLoadBalancer object, and an error if there is any.
func (c *globalLoadBalancers) Get(name string, options v1.GetOptions) (result *v1alpha1.GlobalLoadBalancer, err error) {
	result = &v1alpha1.GlobalLoadBalancer{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("globalloadbalancers").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of GlobalLoadBalancers that match those selectors.
func (c *globalLoadBalancers) List(opts v1.ListOptions) (result *v1alpha1.GlobalLoadBalancerList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.GlobalLoadBalancerList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("globalloadbalancers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested globalLoadBalancers.
func (c *globalLoadBalancers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("globalloadbalancers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch()
}

// Create takes the representation of a globalLoadBalancer and creates it.  Returns the server's representation of the globalLoadBalancer, and an error, if there is any.
func (c *globalLoadBalancers) Create(globalLoadBalancer *v1alpha1.GlobalLoadBalancer) (result *v1alpha1.GlobalLoadBalancer, err error) {
	result = &v1alpha1.GlobalLoadBalancer{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("globalloadbalancers").
		Body(globalLoadBalancer).
		Do().
		Into(result)
	return
}

// Update takes the representation of a globalLoadBalancer and updates it. Returns the server's representation of the globalLoadBalancer, and an error, if there is any.
func (c *globalLoadBalancers) Update(globalLoadBalancer *v1alpha1.GlobalLoadBalancer) (result *v1alpha1.GlobalLoadBalancer, err error) {
	result = &v1alpha1.GlobalLoadBalancer{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("globalloadbalancers").
		Name(globalLoadBalancer.Name).
		Body(globalLoadBalancer).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *globalLoadBalancers) UpdateStatus(globalLoadBalancer *v1alpha1.GlobalLoadBalancer) (result *v1alpha1.GlobalLoadBalancer, err error) {
	result = &v1alpha1.GlobalLoadBalancer{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("globalloadbalancers").
		Name(globalLoadBalancer.Name).
		SubResource("status").
		Body(globalLoadBalancer).
		Do().
		Into(result)
	return
}

// Delete takes name of the globalLoadBalancer and deletes it. Returns an error if one occurs.
func (c *globalLoadBalancers) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("globalloadbalancers").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *globalLoadBalancers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("globalloadbalancers").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched globalLoadBalancer.
func (c *globalLoadBalancers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.GlobalLoadBalancer, err error) {
	result = &v1alpha1.GlobalLoadBalancer{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("globalloadbalancers").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}