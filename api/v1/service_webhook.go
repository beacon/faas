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
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var servicelog = logf.Log.WithName("service-resource")

func (r *Service) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// +kubebuilder:webhook:path=/mutate-extension-ethantang-top-ethantang-top-v1-service,mutating=false,sideEffects=None,admissionReviewVersions=v1,failurePolicy=fail,groups=extension.ethantang.top,resources=services,verbs=create;update,versions=v1,name=mservice.kb.io

var _ webhook.Defaulter = &Service{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Service) Default() {
	servicelog.Info("default", "name", r.Name)

	// TODO: if virtualIP not given, allocate one, perhaps

	// TODO(user): fill in your defaulting logic.
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-extension-ethantang-top-ethantang-top-v1-service,mutating=false,sideEffects=None,admissionReviewVersions=v1,failurePolicy=fail,groups=extension.ethantang.top,resources=services,versions=v1,name=vservice.kb.io

var _ webhook.Validator = &Service{}

func (r *Service) validateService() error {
	if len(r.Spec.TrafficSelectors) == 0 {
		return errors.New("trafficSelectors must be specified")
	}
	if r.Spec.VirtualIP == "" {
		return errors.New("virtualIP must be specified for the time being")
	}
	remainPercent := int32(100)
	for _, traffic := range r.Spec.TrafficSelectors {
		if !(traffic.Percent >= 0 && traffic.Percent <= 100) {
			return errors.New("traffic percent must be [0, 100]")
		}
		remainPercent -= traffic.Percent
	}
	if remainPercent != 0 {
		return errors.New("traffic percent must be sumed total equals to 100")
	}
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Service) ValidateCreate() error {
	servicelog.Info("validate create", "name", r.Name)

	return r.validateService()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Service) ValidateUpdate(old runtime.Object) error {
	servicelog.Info("validate update", "name", r.Name)

	return r.validateService()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Service) ValidateDelete() error {
	servicelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
