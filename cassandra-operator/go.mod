module github.com/sky-uk/cassandra-operator/cassandra-operator

go 1.15

require (
	github.com/PaesslerAG/gval v0.1.1 // indirect
	github.com/PaesslerAG/jsonpath v0.1.0
	github.com/davecgh/go-spew v1.1.1
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/gofrs/flock v0.7.1 // indirect
	github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff // indirect
	github.com/google/go-cmp v0.3.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/common v0.4.1
	github.com/robfig/cron v1.1.0
	github.com/sirupsen/logrus v1.4.2
	github.com/sky-uk/licence-compliance-checker v1.1.1
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.3.0
	github.com/theckman/go-flock v0.7.0
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/zap v1.10.0 // indirect
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45 // indirect
	golang.org/x/time v0.0.0-20181108054448-85acf8d2951c // indirect
	golang.org/x/tools v0.0.0-20190621195816-6e04913cbbac
	gopkg.in/src-d/go-license-detector.v2 v2.0.1 // indirect
	k8s.io/api v0.0.0-20190918195907-bd6ac527cfd2
	k8s.io/apimachinery v0.0.0-20190913080033-27d36303b655
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/code-generator v0.0.0-20190912054826-cd179ad6a269
	sigs.k8s.io/controller-runtime v0.3.0
	sigs.k8s.io/controller-tools v0.2.2
)

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190619194433-921a716ae8da

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190416092415-3370b4aef5d6

replace gomodules.xyz/jsonpatch/v2 => github.com/gomodules/jsonpatch/v2 v2.0.1+incompatible

replace gotest.tools => github.com/gotestyourself/gotest.tools v2.2.0+incompatible
