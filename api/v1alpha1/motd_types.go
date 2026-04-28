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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MotdSpec defines the desired state of Motd.
type MotdSpec struct {
	// Message is the text to display via pod names
	Message string `json:"message"`

	// Intensity controls pod behavior: "" for stable, "crashloop" for crashing pods (shows red in k9s)
	Intensity string `json:"intensity,omitempty"`
}

// MotdStatus defines the observed state of Motd.
type MotdStatus struct {
	// PodNames contains the names of pods currently displaying the message
	PodNames []string `json:"podNames,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Motd is the Schema for the motds API.
type Motd struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MotdSpec   `json:"spec,omitempty"`
	Status MotdStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MotdList contains a list of Motd.
type MotdList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Motd `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Motd{}, &MotdList{})
}
