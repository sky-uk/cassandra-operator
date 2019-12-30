module github.com/sky-uk/cassandra-operator/cassandra-snapshot

go 1.12

require (
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20190421051319-9d40249d3c2f // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20190421051319-9d40249d3c2f // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/sirupsen/logrus v1.4.2
	github.com/sky-uk/cassandra-operator/cassandra-operator v0.0.0-20190626140523-86a852951aa5
	github.com/sky-uk/licence-compliance-checker v1.1.1
	github.com/spf13/cobra v0.0.5
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422
	k8s.io/api v0.0.0-20190918195907-bd6ac527cfd2
	k8s.io/apimachinery v0.0.0-20190913080033-27d36303b655
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
)

replace github.com/sky-uk/cassandra-operator/cassandra-operator => ../cassandra-operator

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190619194433-921a716ae8da

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190416092415-3370b4aef5d6

replace gomodules.xyz/jsonpatch/v2 => github.com/gomodules/jsonpatch/v2 v2.0.1+incompatible

replace gotest.tools => github.com/gotestyourself/gotest.tools v2.2.0+incompatible
