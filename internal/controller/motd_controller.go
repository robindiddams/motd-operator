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

package controller

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	motdv1alpha1 "github.com/robindiddams/motd-operator/api/v1alpha1"
)

// MotdReconciler reconciles a Motd object
type MotdReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=motd.howcoldismy.beer,resources=motds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=motd.howcoldismy.beer,resources=motds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=motd.howcoldismy.beer,resources=motds/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete

const (
	maxPodNameLen = 58
	labelKey      = "motd.howcoldismy.beer/owner"

	headerPodName = "00-00-00-00-00-00-00-message-of-the-day-00-00-00-00-00-00"
	footerPodName = "99-00-00-00-00-00-00-00-00-00-00-00-00-00-00-00-00-00-00"
)

// sanitize converts a string to a valid DNS label (lowercase a-z0-9-, no leading/trailing/consecutive dashes)
func sanitize(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(s, "-") // replace invalid chars with dash
	s = regexp.MustCompile(`-+`).ReplaceAllString(s, "-")          // collapse consecutive dashes
	s = strings.Trim(s, "-")                                       // trim leading/trailing dashes
	return s
}

func encodeMessage(message string) []string {
	if message == "" {
		return nil
	}

	// Handle escaped newlines
	message = strings.ReplaceAll(message, "\\n", "\n")

	// Split by newlines for multi-line support
	lines := strings.Split(message, "\n")

	var podNames []string

	// Add header
	podNames = append(podNames, headerPodName)

	for lineIdx, line := range lines {
		words := strings.Fields(line) // split by whitespace
		var chunk strings.Builder

		for _, word := range words {
			word = sanitize(word)
			if word == "" {
				continue
			}
			// Line numbers start at 01 (header is 00)
			prefix := fmt.Sprintf("%02d-", lineIdx+1)
			proposed := chunk.String() + "-" + word
			if len(prefix)+len(proposed) > maxPodNameLen {
				// Flush current chunk
				if chunk.Len() > 0 {
					podNames = append(podNames, prefix+chunk.String())
				}
				chunk.Reset()
			}
			if chunk.Len() > 0 {
				chunk.WriteString("-")
			}
			chunk.WriteString(word)
		}

		// Flush remaining chunk
		if chunk.Len() > 0 {
			prefix := fmt.Sprintf("%02d-", lineIdx+1)
			podNames = append(podNames, prefix+chunk.String())
		}
	}

	// Add footer
	podNames = append(podNames, footerPodName)

	return podNames
}

func (r *MotdReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the Motd instance
	motd := &motdv1alpha1.Motd{}
	if err := r.Get(ctx, req.NamespacedName, motd); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Motd resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Motd")
		return ctrl.Result{}, err
	}

	// Encode message to pod names
	desiredPodNames := encodeMessage(motd.Spec.Message)

	// List existing pods owned by this Motd
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(req.Namespace), client.MatchingLabels{labelKey: motd.Name}); err != nil {
		log.Error(err, "Failed to list pods")
		return ctrl.Result{}, err
	}

	existingPods := make(map[string]*corev1.Pod)
	for i := range podList.Items {
		existingPods[podList.Items[i].Name] = &podList.Items[i]
	}

	// Calculate desired pod names as a set
	desiredSet := make(map[string]bool)
	for _, name := range desiredPodNames {
		desiredSet[name] = true
	}

	// Delete pods that are no longer needed
	for name, pod := range existingPods {
		if !desiredSet[name] {
			log.Info("Deleting obsolete pod", "pod", name)
			if err := r.Delete(ctx, pod); err != nil && !errors.IsNotFound(err) {
				log.Error(err, "Failed to delete pod", "pod", name)
				return ctrl.Result{}, err
			}
		}
	}

	// Create pods that don't exist yet
	for _, podName := range desiredPodNames {
		if _, exists := existingPods[podName]; !exists {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: req.Namespace,
					Labels: map[string]string{
						labelKey: motd.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "display",
							Image: "registry.k8s.io/pause:3.1",
						},
					},
				},
			}

			// Apply crashloop intensity if specified
			if motd.Spec.Intensity == "crashloop" {
				pod.Spec.Containers[0].Image = "busybox"
				pod.Spec.Containers[0].Command = []string{"sh", "-c", "exit 1"}
			}

			// Set owner reference for garbage collection
			if err := controllerutil.SetControllerReference(motd, pod, r.Scheme); err != nil {
				log.Error(err, "Failed to set owner reference")
				return ctrl.Result{}, err
			}

			log.Info("Creating pod", "pod", podName)
			if err := r.Create(ctx, pod); err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "Failed to create pod", "pod", podName)
				return ctrl.Result{}, err
			}
		}
	}

	// Update status
	motd.Status.PodNames = desiredPodNames
	if err := r.Status().Update(ctx, motd); err != nil {
		log.Error(err, "Failed to update Motd status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MotdReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&motdv1alpha1.Motd{}).
		Named("motd").
		Complete(r)
}
