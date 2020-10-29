module k8c.io/kubelb

go 1.13

replace k8c.io/kubelb/manager => ./manager

replace k8c.io/kubelb/agent => ./agent

require (
	k8c.io/kubelb/manager v0.0.0-00010101000000-000000000000 // indirect
	k8s.io/code-generator v0.19.3
)
