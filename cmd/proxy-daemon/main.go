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

package main

import (
	"flag"
	"log"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	extensionv1 "github.com/beacon/faas/api/v1"
	"github.com/beacon/faas/controllers"
	"github.com/beacon/faas/pkg/ipvs"
	"k8s.io/utils/exec"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = extensionv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

// Options for program
type Options struct {
	MasterDevice   string
	ClusterIPRange string
}

func main() {
	var opts Options
	var metricsAddr string
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&opts.MasterDevice, "masterDevice", "eth0", "Master device name")
	flag.StringVar(&opts.ClusterIPRange, "clusterIPRange", "", "IP range for created services")

	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     false,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	dev := ipvs.NewDevice(ipvs.DummyDeviceName, opts.MasterDevice)
	if err := dev.EnsureDevice(); err != nil {
		log.Fatalln("failed to ensure device:", err)
	}

	execer := exec.New()
	ipvsInterface := ipvs.New(execer)

	if err = (&controllers.ServiceReconciler{
		IpvsInterface: ipvsInterface,
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("Service"),
		Scheme:        mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Service")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
