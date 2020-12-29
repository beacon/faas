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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionv1 "github.com/beacon/faas/api/v1"
)

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const finalizerName = "storage.finalizers.ethantang.top"

// Reconcile r
// +kubebuilder:rbac:groups=extension.ethantang.top,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=extension.ethantang.top,resources=services/status,verbs=get;update;patch
func (r *ServiceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("service", req.NamespacedName)

	var svc extensionv1.Service
	if err := r.Get(ctx, req.NamespacedName, &svc); err != nil {
		log.V(4).Info("failed to get Service", "service", req.NamespacedName, "error", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if svc.ObjectMeta.DeletionTimestamp.IsZero() {
		// add/update, inject finalizer
		if !hasFinalizer(svc.ObjectMeta.Finalizers) {
			log.V(4).Info("injected finalizer", "service", req.NamespacedName)
			svc.ObjectMeta.Finalizers = addFinalizer(svc.ObjectMeta.Finalizers)
			if err := r.Update(ctx, &svc); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// Under deletion
		if hasFinalizer(svc.ObjectMeta.Finalizers) {
			log.V(4).Info("do deletion jobs for", "service", req.NamespacedName)
			svc.ObjectMeta.Finalizers = removeFinalizer(svc.ObjectMeta.Finalizers)
			if err := r.Update(ctx, &svc); err != nil {
				return ctrl.Result{}, nil
			}
		}

		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&extensionv1.Service{}).
		Complete(r)
}

func hasFinalizer(s []string) bool {
	for _, item := range s {
		if item == finalizerName {
			return true
		}
	}
	return false
}

func addFinalizer(s []string) (result []string) {
	return append(s, finalizerName)
}

func removeFinalizer(s []string) (result []string) {
	for _, item := range s {
		if item == finalizerName {
			continue
		}
		result = append(result, item)
	}
	return
}
