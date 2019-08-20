module github.com/sky-uk/cassandra-operator/cassandra-operator

go 1.12

require (
	github.com/PaesslerAG/gval v0.1.1 // indirect
	github.com/PaesslerAG/jsonpath v0.1.0
	github.com/davecgh/go-spew v1.1.1
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/gofrs/flock v0.7.1 // indirect
	github.com/gogo/protobuf v1.2.2-0.20190723190241-65acae22fc9d // indirect
	github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff // indirect
	github.com/golang/protobuf v1.3.1 // indirect
	github.com/google/go-cmp v0.3.0
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.1
	github.com/prometheus/common v0.1.0
	github.com/prometheus/procfs v0.0.0-20190104112138-b1a0a9a36d74 // indirect
	github.com/robfig/cron v1.1.0
	github.com/sirupsen/logrus v1.3.0
	github.com/sky-uk/licence-compliance-checker v1.1.1
	github.com/spf13/cobra v0.0.3
	github.com/stretchr/testify v1.3.0
	github.com/theckman/go-flock v0.7.0
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/zap v1.10.0 // indirect
	golang.org/x/crypto v0.0.0-20190611184440-5c40567a22f8 // indirect
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422
	golang.org/x/net v0.0.0-20190613194153-d28f0bde5980 // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45 // indirect
	golang.org/x/sys v0.0.0-20190616124812-15dcb6c0061f // indirect
	golang.org/x/time v0.0.0-20181108054448-85acf8d2951c // indirect
	golang.org/x/tools v0.0.0-20190614205625-5aca471b1d59
	gopkg.in/src-d/go-license-detector.v2 v2.0.1 // indirect
	k8s.io/api v0.0.0-20190619194126-8359267a0ae8
	k8s.io/apiextensions-apiserver v0.0.0-20190409022649-727a075fdec8
	k8s.io/apimachinery v0.0.0-20190416092415-3370b4aef5d6
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/code-generator v0.0.0-20190416052311-01a054e913a9
	k8s.io/klog v0.3.1 // indirect
	k8s.io/kube-openapi v0.0.0-20190709113604-33be087ad058 // indirect
	sigs.k8s.io/controller-runtime v0.2.0-beta.0
	sigs.k8s.io/controller-tools v0.2.0-beta.0
	sigs.k8s.io/yaml v1.1.0
)

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190619194433-921a716ae8da

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190416092415-3370b4aef5d6

replace sigs.k8s.io/controller-tools => sigs.k8s.io/controller-tools v0.2.0-beta.3

replace sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.2.0-beta.3
