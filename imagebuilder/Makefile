all: gocode

gocode:
	GO111MODULE=on go install k8s.io/kube-deploy/imagebuilder

gofmt:
	gofmt -w -s main.go
	gofmt -w -s pkg
