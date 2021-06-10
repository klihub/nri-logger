module github.com/klihub/nri-test-plugin

go 1.16

require (
	github.com/containerd/nri v0.0.0-20210324153033-efa1ddcc27b4
	github.com/containerd/ttrpc v1.0.2 // indirect
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1 // indirect
	golang.org/x/sys v0.0.0-20210514084401-e8d321eab015 // indirect
	google.golang.org/genproto v0.0.0-20210315173758-2651cd453018 // indirect
	google.golang.org/grpc v1.36.0 // indirect
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/containerd/nri v0.0.0-20210324153033-efa1ddcc27b4 => ../nri
