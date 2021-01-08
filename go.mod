module github.com/beacon/faas

go 1.13

require (
	github.com/go-logr/logr v0.3.0
	github.com/moby/ipvs v1.0.1
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/pkg/errors v0.9.1
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/sys v0.0.0-20200622214017-ed371f2e16b4
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	k8s.io/klog/v2 v2.4.0 // indirect
	k8s.io/utils v0.0.0-20200912215256-4140de9c8800
	sigs.k8s.io/controller-runtime v0.7.0
)
