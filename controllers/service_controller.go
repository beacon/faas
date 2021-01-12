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
	"fmt"
	"log"
	"net"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
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

type svcMetaMap map[types.NamespacedName]serviceTrafficSelector

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	Device        ipvs.Device
	IpvsInterface ipvs.Interface
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	svcMetaMap svcMetaMap
}

type serviceTrafficSelector struct {
	resourceVersion  string
	trafficSelectors []extensionv1.TrafficSelector
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
	if !svc.ObjectMeta.DeletionTimestamp.IsZero() {
		// Under deletion
		if hasFinalizer(svc.ObjectMeta.Finalizers) {
			log.V(4).Info("do deletion jobs for", "service", req.NamespacedName)

			if err := r.Device.UnbindAddress(svc.Spec.VirtualIP); err != nil {
				log.Error(err, "failed to unbind address", "address", svc.Spec.VirtualIP)
			}

			vss := ipvs.ServiceToVirtualServers(&svc)
			for _, vs := range vss {
				if err := r.IpvsInterface.DeleteVirtualServer(vs); err != nil {
					log.Error(err, "failed to delete virtual server", "svc", svc.Name, "namespace", svc.Namespace)
				}
			}

			svc.ObjectMeta.Finalizers = removeFinalizer(svc.ObjectMeta.Finalizers)
			if err := r.Update(ctx, &svc); err != nil {
				return ctrl.Result{}, nil
			}
		}

		return ctrl.Result{}, nil

	}

	// add/update, inject finalizer
	if !hasFinalizer(svc.ObjectMeta.Finalizers) {
		log.V(4).Info("injected finalizer", "service", req.NamespacedName)
		svc.ObjectMeta.Finalizers = addFinalizer(svc.ObjectMeta.Finalizers)
		if err := r.Update(ctx, &svc); err != nil {
			return ctrl.Result{}, err
		}
	}
	storedMeta, ok := r.svcMetaMap[req.NamespacedName]
	if !ok || storedMeta.resourceVersion != svc.ResourceVersion {
		r.svcMetaMap[req.NamespacedName] = serviceTrafficSelector{
			resourceVersion:  svc.ResourceVersion,
			trafficSelectors: svc.Spec.TrafficSelectors,
		}

		exists, err := r.Device.EnsureAddressBind(svc.Spec.VirtualIP)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to bind virtual ip")
		}
		if exists {
			log.V(4).Info("virtualIP already bound", "ip", svc.Spec.VirtualIP)
		}

		const initialWeight = 1000
		const magnifyFactor = 10
		// magnified with 10
		remainWeight := initialWeight
		// build supposed real servers
		// real server map to avoid duplicate pods in weight allocation
		realServerMap := make(map[string]*ipvs.RealServer)

		prepareRealServer := func(selector map[string]string, percent int) error {
			if percent == 0 {
				return nil
			}
			podList := new(corev1.PodList)
			err := r.List(ctx, podList, &client.ListOptions{
				LabelSelector: labels.Set(selector).AsSelector(),
			})
			if err != nil {
				return errors.Wrap(err, "failed to list pods")
			}
			// No available weights
			if len(podList.Items) == 0 {
				return nil
			}
			availWeight := percent * magnifyFactor / 100
			weightPerPod := availWeight / len(podList.Items)
			for _, pod := range podList.Items {
				if !isPodReady(&pod) {
					continue
				}
				for _, svcPort := range svc.Spec.Ports {
					portNum, err := podutil.FindPort(&pod, &svcPort)
					if err != nil {
						log.Info("failed to get port",
							"servicePort", svcPort.TargetPort,
							"namespace", pod.Namespace,
							"pod", pod.Name)
						continue
					}
					podKey := fmt.Sprintf("%s:%d", pod.Status.PodIP, portNum)
					rs, ok := realServerMap[podKey]
					if !ok {
						rs = &ipvs.RealServer{
							Address: net.ParseIP(pod.Status.PodIP),
							Port:    uint16(portNum),
							Weight:  weightPerPod,
						}
						realServerMap[podKey] = rs
					} else {
						rs.Weight += weightPerPod
					}
				}
			}
			return nil
		}
		for _, trafficSelector := range svc.Spec.TrafficSelectors {
			err := prepareRealServer(trafficSelector.Selector, int(trafficSelector.Percent))
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to list pods")
			}
		}
		// No pod available, may need failover
		if remainWeight == initialWeight && len(svc.Spec.FailoverSelector) != 0 {
			if err := prepareRealServer(svc.Spec.FailoverSelector, 100); err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to prepare failover pods")
			}
		}

		vss := ipvs.ServiceToVirtualServers(&svc)
		for i, vs := range vss {
			actualVS, err := r.IpvsInterface.GetVirtualServer(vs)
			if err != nil {
				// Try create one vs
				if err := r.IpvsInterface.AddVirtualServer(vs); err != nil {
					return ctrl.Result{}, errors.Wrap(err, "failed to add virtual service")
				}
			} else {
				vss[i] = actualVS
			}
			existingRS, err := r.IpvsInterface.GetRealServers(vs)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to get real servers")
			}
			for _, rs := range existingRS {
				if err := r.IpvsInterface.DeleteRealServer(vs, rs); err != nil {
					return ctrl.Result{}, errors.Wrap(err, "failed to delete real server")
				}
			}
			for _, rs := range realServerMap {
				if err := r.IpvsInterface.AddRealServer(vs, rs); err != nil {
					return ctrl.Result{}, errors.Wrap(err, "failed to add real server")
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.svcMetaMap = make(svcMetaMap)
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

func isPodReady(pod *corev1.Pod) bool {
	if !podutil.IsPodReady(pod) {
		return false
	}
	if pod.Status.PodIP == "" {
		return false
	}
	return true
}

func (r *ServiceReconciler) handlePodChange(namespace, name string) error {
	var pod corev1.Pod
	ctx := context.Background()
	podNS := types.NamespacedName{Namespace: namespace, Name: name}
	err := r.Get(ctx, podNS, &pod)
	if err != nil {
		return errors.Wrap(err, "failed to get pod")
	}
	if !isPodReady(&pod) {
		return nil
	}
	log := r.Log.WithValues("pod", podNS)
	podIP := net.ParseIP(pod.Status.PodIP)
	for item, trafficSelectors := range r.svcMetaMap {
		for _, ts := range trafficSelectors.trafficSelectors {
			matchTraffic := true
			for key, value := range ts.Selector {
				if pod.Labels[key] != value {
					matchTraffic = false
					break
				}
			}
			if !matchTraffic {
				break
			}

			var svc extensionv1.Service
			if err := r.Get(ctx, item, &svc); err != nil {
				return errors.Wrap(err, "failed to get service")
			}
			// When a pod is created/updated, two things to handle:
			// 1. Should it be added or updated?
			// 2. Maybe we can change the weight allocation in need?

			// When a pod's corresponding real server need to be updated,
			// either it's newly added or removed
			err = ipvs.RangeServiceVirtualServers(&svc, func(vs *ipvs.VirtualServer, svcPort *corev1.ServicePort) error {
				remainWeight := 1000
				rss, err := r.IpvsInterface.GetRealServers(vs)
				if err != nil {
					return errors.Wrap(err, "failed to get real servers")
				}
				for _, rs := range rss {
					if rs.Address.Equal(podIP) {
						portNum, err := podutil.FindPort(&pod, svcPort)
						if err != nil {
							log.Info("failed to get port",
								"servicePort", svcPort.TargetPort,
								"namespace", pod.Namespace,
								"pod", pod.Name)
							continue
						}
						rs = &ipvs.RealServer{
							Address: net.ParseIP(pod.Status.PodIP),
							Port:    uint16(portNum),
							Weight:  weightPerPod,
						}
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
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
	if !podutil.IsPodReady(&pod) || pod.Status.PodIP == "" {
		return
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
