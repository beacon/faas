/*


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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ServiceSpec defines the desired state of Service
type ServiceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Ports []corev1.ServicePort `json:"ports,omitempty"`

	// VirtualIP manually given for the time being
	// TODO: make it automatically allocated.
	// Native kubernetes services are allocated from pool, avoiding such issue,
	// but we have to take care of IP collision.
	VirtualIP string `json:"virtualIP"`
	// LoadBalanceMethod defines how to load balance between endpoints
	// Available methods are defined in ipvs:
	LoadBalanceMethod string `json:"loadBalanceMethod"`
	// TrafficSelectors deeper selects traffic and distribute to sub endpoints
	// Percent must be sumed up as 100 (TODO: may not necessary)
	// Endpoints should not overlap
	// +optional
	TrafficSelectors []TrafficSelector `json:"trafficSelectors,omitempty"`

	// FailoverSelector works when all endpoints of trafficSelectors are down.
	// It will turn to be active with weight 1
	FailoverSelector map[string]string `json:"failoverSelector,omitempty"`
}

// TrafficSelector to select specified endpoints and allocate traffic
type TrafficSelector struct {
	Selector map[string]string `json:"selector"`
	Percent  int32             `json:"percent,omitempty"`
}

// ServiceStatus defines the observed state of Service
type ServiceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// Service is the Schema for the services API
type Service struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceSpec   `json:"spec,omitempty"`
	Status ServiceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceList contains a list of Service
type ServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Service `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Service{}, &ServiceList{})
}
