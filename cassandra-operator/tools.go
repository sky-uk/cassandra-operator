// see https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
// and https://github.com/golang/go/issues/25922

// +build tools

package tools

import (
	_ "github.com/onsi/ginkgo/ginkgo"
	_ "github.com/sky-uk/licence-compliance-checker"
	_ "golang.org/x/lint/golint"
	_ "golang.org/x/tools/cmd/goimports"
	_ "k8s.io/code-generator"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
