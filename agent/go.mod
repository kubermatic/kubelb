module agent

go 1.13

replace k8c.io/kubelb/manager => ../manager

require (
	github.com/go-logr/logr v0.1.0
	k8c.io/kubelb/manager v0.0.0-00010101000000-000000000000
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	sigs.k8s.io/controller-runtime v0.5.0
)
