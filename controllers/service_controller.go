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
	"log"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	extensionv1 "github.com/beacon/faas/api/v1"
	"github.com/beacon/faas/pkg/ipvs"
	"github.com/beacon/faas/pkg/utils/podutil"
)

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	IpvsInterface ipvs.Interface
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const finalizerName = "storage.finalizers.ethantang.top"

// Reconcile r
// +kubebuilder:rbac:groups=extension.ethantang.top,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=extension.ethantang.top,resources=services/status,verbs=get;update;patch
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
		For(&extensionv1.Service{}).Watches(
		&source.Kind{
			Type: &corev1.Pod{},
		},
		&handler.Funcs{
			CreateFunc:  r.onPodCreate,
			UpdateFunc:  r.onPodUpdate,
			DeleteFunc:  r.onPodDelete,
			GenericFunc: r.onPodGeneric,
		}).
		Complete(r)
}

func (r *ServiceReconciler) onPodCreate(event event.CreateEvent, queue workqueue.RateLimitingInterface) {
	log.Println("podcreated:", event.Object.GetName())
	var pod corev1.Pod
	err := r.Get(context.Background(), types.NamespacedName{
		Namespace: event.Object.GetNamespace(),
		Name:      event.Object.GetName()}, &pod)
	if err != nil {
		log.Println("failed to get pod:", err)
	}
	if podutil.IsPodReady(&pod) {
		log.Println("Pod is ready:", pod.Status.PodIP)
	}
}

func (r *ServiceReconciler) onPodUpdate(event event.UpdateEvent, queue workqueue.RateLimitingInterface) {
	log.Println("podupdated:", event.ObjectNew.GetName())
}

func (r *ServiceReconciler) onPodDelete(event event.DeleteEvent, queue workqueue.RateLimitingInterface) {
	log.Println("poddeleted:", event.Object.GetName())
}

func (r *ServiceReconciler) onPodGeneric(event event.GenericEvent, queue workqueue.RateLimitingInterface) {
	log.Println("podgeneric event:", event.Object.GetName())
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
