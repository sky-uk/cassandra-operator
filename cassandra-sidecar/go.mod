module github.com/sky-uk/cassandra-operator/cassandra-sidecar

go 1.12

require (
	github.com/googleapis/gnostic v0.3.0 // indirect
	github.com/onsi/ginkgo v1.8.0
	github.com/sirupsen/logrus v1.4.2
	github.com/sky-uk/cassandra-operator/cassandra-operator v0.0.0-20190607105530-f2a6996272c3
	github.com/sky-uk/licence-compliance-checker v1.1.1
	github.com/spf13/cobra v0.0.4
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422
	golang.org/x/tools v0.0.0-20190606174628-0139d5756a7d
	k8s.io/apimachinery v0.0.0-20190606174813-5a6182816fbf
	k8s.io/utils v0.0.0-20190607212802-c55fbcfc754a // indirect
)

replace github.com/sky-uk/cassandra-operator/cassandra-operator => ../cassandra-operator

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190216013122-f05b8decd79c
