package metrics

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/apis"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	metricstesting "github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/metrics/testing"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/stub"
)

var _ = Describe("Cluster Metrics", func() {
	var (
		jolokiaURLProvider *metricstesting.StubbedJolokiaURLProvider
		metricsGatherer    Gatherer
		cluster            *cluster.Cluster
	)

	BeforeEach(func() {
		jolokia.responsePrimers = make(map[string]jolokiaResponsePrimer)

		jolokia.returnsNoLiveNodes()
		jolokia.returnsNoUnreachableNodes()
		jolokia.returnsNoJoiningNodes()
		jolokia.returnsNoLeavingNodes()
		jolokia.returnsNoMovingNodes()
		jolokia.returnsRackForNode("racka", "172.16.46.58")
		jolokia.returnsRackForNode("racka", "172.16.101.30")

		jolokiaURLProvider = &metricstesting.StubbedJolokiaURLProvider{BaseURL: serverURL}
		metricsGatherer = NewGatherer(jolokiaURLProvider, &Config{1 * time.Second})

		cluster = aCluster("testcluster", "test")
	})

	Context("A cluster has been defined", func() {
		It("gathers the live node count metric", func() {
			// given
			jolokia.returns2LiveNodes()

			// when
			clusterStatus, err := metricsGatherer.GatherMetricsFor(cluster)

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(clusterStatus.liveNodes).To(ConsistOf("172.16.46.58", "172.16.101.30"))
		})

		It("gathers the unreachable node count metric", func() {
			// given
			jolokia.returns2UnreachableNodes()

			// when
			clusterStatus, err := metricsGatherer.GatherMetricsFor(cluster)

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(clusterStatus.unreachableNodes).To(ConsistOf("172.16.46.58", "172.16.101.30"))
		})

		It("gathers the joining node count metric", func() {
			// given
			jolokia.returns2JoiningNodes()

			// when
			clusterStatus, err := metricsGatherer.GatherMetricsFor(cluster)

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(clusterStatus.joiningNodes).To(ConsistOf("172.16.46.58", "172.16.101.30"))
		})

		It("gathers the leaving node count metric", func() {
			// given
			jolokia.returns2LeavingNodes()

			// when
			clusterStatus, err := metricsGatherer.GatherMetricsFor(cluster)

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(clusterStatus.leavingNodes).To(ConsistOf("172.16.46.58", "172.16.101.30"))
		})

		It("gathers the moving node count metric", func() {
			// given
			jolokia.returns2MovingNodes()

			// when
			clusterStatus, err := metricsGatherer.GatherMetricsFor(cluster)

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(clusterStatus.movingNodes).To(ConsistOf("172.16.46.58", "172.16.101.30"))
		})

		It("returns an error when jolokia is not available", func() {
			// given
			jolokiaURLProvider.JolokiaIsUnavailable()

			// when
			_, err := metricsGatherer.GatherMetricsFor(cluster)

			// then
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when jolokia returns an error response", func() {
			// given
			jolokia.returnsErrorResponse()

			// when
			_, err := metricsGatherer.GatherMetricsFor(cluster)

			// then
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when jolokia returns an non json response", func() {
			// given
			jolokia.returnsANonJSONResponse()

			// when
			_, err := metricsGatherer.GatherMetricsFor(cluster)

			// then
			Expect(err).To(HaveOccurred())
		})

		It("retrieves rack details for each node", func() {
			// given
			jolokia.returnsLiveNodes("172.0.0.1", "172.0.0.2")
			jolokia.returnsUnreachableNodes("172.0.0.3", "172.0.0.4")
			jolokia.returnsRackForNode("racka", "172.0.0.1")
			jolokia.returnsRackForNode("racka", "172.0.0.4")
			jolokia.returnsRackForNode("rackb", "172.0.0.2")
			jolokia.returnsRackForNode("rackb", "172.0.0.3")

			// when
			clusterStatus, err := metricsGatherer.GatherMetricsFor(cluster)

			// then
			Expect(err).To(Not(HaveOccurred()))
			Expect(clusterStatus.nodeRacks).To(And(
				HaveLen(4),
				HaveKeyWithValue("172.0.0.1", "racka"),
				HaveKeyWithValue("172.0.0.4", "racka"),
				HaveKeyWithValue("172.0.0.2", "rackb"),
				HaveKeyWithValue("172.0.0.3", "rackb"),
			))

		})
	})
})

var _ = Describe("Metrics URL randomisation", func() {
	var cluster *cluster.Cluster
	var standardPodLabels map[string]string

	BeforeEach(func() {
		cluster = aCluster("testcluster", "test")
		standardPodLabels = make(map[string]string)
		standardPodLabels["app.kubernetes.io/instance"] = "testcluster"
	})

	It("should return a different pod URL each time it is invoked", func() {
		// given
		podsGetter := stub.NewStubbedPodsGetter(standardPodLabels, "10.0.0.1", "10.0.0.2")
		urlProvider := &randomisingJolokiaURLProvider{podsGetter, rand.New(rand.NewSource(0))}

		// when
		urlsProvided := make(map[string]int)
		for i := 0; i < 10; i++ {
			urlProvided := urlProvider.URLFor(cluster)
			urlsProvided[urlProvided] = 1
		}

		// then
		Expect(urlsProvided).To(HaveLen(2))
		Expect(urlsProvided).To(HaveKey("http://10.0.0.1:7777"))
		Expect(urlsProvided).To(HaveKey("http://10.0.0.2:7777"))
	})

	It("should return the service URL if there is a problem listing pods", func() {
		// given
		podsGetter := stub.NewFailingStubbedPodsGetter()
		urlProvider := &randomisingJolokiaURLProvider{podsGetter, rand.New(rand.NewSource(0))}

		// when
		urlProvided := urlProvider.URLFor(cluster)

		// then
		Expect(urlProvided).To(Equal("http://testcluster.test:7777"))
	})

	It("should return the service URL if no pod is found", func() {
		// given
		podsGetter := stub.NewStubbedPodsGetter(nil)
		urlProvider := &randomisingJolokiaURLProvider{podsGetter, rand.New(rand.NewSource(0))}

		// when
		urlProvided := urlProvider.URLFor(cluster)

		// then
		Expect(urlProvided).To(Equal("http://testcluster.test:7777"))
	})

	Context("when some pods do not have an IP address", func() {
		It("should use only pods with an IP address", func() {
			// given
			podsGetter := stub.NewStubbedPodsGetter(standardPodLabels, "10.0.0.1", "")
			urlProvider := &randomisingJolokiaURLProvider{podsGetter, rand.New(rand.NewSource(0))}

			for i := 0; i < 10; i++ {
				// when
				urlProvided := urlProvider.URLFor(cluster)

				// then
				Expect(urlProvided).To(Equal("http://10.0.0.1:7777"))
			}
		})

		It("should return the service URL if no pods have an IP address", func() {
			// given
			podsGetter := stub.NewStubbedPodsGetter(standardPodLabels, "", "")
			urlProvider := &randomisingJolokiaURLProvider{podsGetter, rand.New(rand.NewSource(0))}

			// when
			urlProvided := urlProvider.URLFor(cluster)

			// then
			Expect(urlProvided).To(Equal("http://testcluster.test:7777"))
		})

		It("should return the service URL if no pods have the instance label set to the clustername", func() {
			// given
			labels := make(map[string]string)
			podsGetter := stub.NewStubbedPodsGetter(labels, "10.0.0.1", "10.0.0.2")
			urlProvider := &randomisingJolokiaURLProvider{podsGetter, rand.New(rand.NewSource(0))}

			// when
			urlProvided := urlProvider.URLFor(cluster)

			// then
			Expect(urlProvided).To(Equal("http://testcluster.test:7777"))
		})
	})
})

type jolokiaResponsePrimer struct {
	response   string
	statusCode int
}

type jolokiaHandler struct {
	responsePrimers map[string]jolokiaResponsePrimer
}

func (jh *jolokiaHandler) returnsErrorResponse() {
	// This is a legitimate situation - Jolokia does return errors with a 200 code in the HTTP status header and
	// a "proper" error status code in its JSON response body!
	jh.responsePrimers["LiveNodes"] = jolokiaResponsePrimer{
		response: `{
		"stacktrace": "java.lang.IllegalArgumentException: Invalid JSON request java.io.InputStreamReader@4a1202c\n\tat org.jolokia.http.HttpRequestHandler.extractJsonRequest(HttpRequestHandler.java:181)\n\tat org.jolokia.http.HttpRequestHandler.handlePostRequest(HttpRequestHandler.java:121)\n\tat org.jolokia.jvmagent.handler.JolokiaHttpHandler.executePostRequest(JolokiaHttpHandler.java:290)\n\tat org.jolokia.jvmagent.handler.JolokiaHttpHandler.doHandle(JolokiaHttpHandler.java:236)\n\tat org.jolokia.jvmagent.handler.JolokiaHttpHandler.handle(JolokiaHttpHandler.java:178)\n\tat com.sun.net.httpserver.Filter$Chain.doFilter(Filter.java:79)\n\tat sun.net.httpserver.AuthFilter.doFilter(AuthFilter.java:83)\n\tat com.sun.net.httpserver.Filter$Chain.doFilter(Filter.java:82)\n\tat sun.net.httpserver.ServerImpl$Exchange$LinkHandler.handle(ServerImpl.java:675)\n\tat com.sun.net.httpserver.Filter$Chain.doFilter(Filter.java:79)\n\tat sun.net.httpserver.ServerImpl$Exchange.run(ServerImpl.java:647)\n\tat java.util.concurrent.ThreadPoolExecutor.runWorker(ThreadPoolExecutor.java:1149)\n\tat java.util.concurrent.ThreadPoolExecutor$Worker.run(ThreadPoolExecutor.java:624)\n\tat java.lang.Thread.run(Thread.java:748)\nCaused by: Unexpected token END OF FILE at position 0.\n\tat org.json.simple.parser.JSONParser.parse(Unknown Source)\n\tat org.json.simple.parser.JSONParser.parse(Unknown Source)\n\tat org.jolokia.http.HttpRequestHandler.extractJsonRequest(HttpRequestHandler.java:179)\n\t... 13 more\n",
		"error_type": "java.lang.IllegalArgumentException",
		"error": "java.lang.IllegalArgumentException : Invalid JSON request java.io.InputStreamReader@4a1202c",
		"status": 400
	}`,
		statusCode: 200,
	}
}

func (jh *jolokiaHandler) returnNodesForMbean(mbean string, nodeIPs ...string) {
	var nodeIPJsonValue []string
	for _, nodeIP := range nodeIPs {
		nodeIPJsonValue = append(nodeIPJsonValue, fmt.Sprintf("\"%s\"", nodeIP))
	}

	jh.responsePrimers[mbean] = jolokiaResponsePrimer{
		response: fmt.Sprintf(`{
  "request": {
	"mbean": "org.apache.cassandra.db:type=StorageService",
	"attribute": "%s",
	"type": "read"
  },
  "value": [
	%s
  ],
  "timestamp": 1524056270,
  "status": 200
}`, mbean, strings.Join(nodeIPJsonValue, ",")),
		statusCode: 200,
	}
}

func (jh *jolokiaHandler) returns2LiveNodes() {
	jh.returnNodesForMbean("LiveNodes", "172.16.46.58", "172.16.101.30")
}

func (jh *jolokiaHandler) returns2UnreachableNodes() {
	jh.returnNodesForMbean("UnreachableNodes", "172.16.46.58", "172.16.101.30")
}

func (jh *jolokiaHandler) returns2JoiningNodes() {
	jh.returnNodesForMbean("JoiningNodes", "172.16.46.58", "172.16.101.30")
}

func (jh *jolokiaHandler) returns2LeavingNodes() {
	jh.returnNodesForMbean("LeavingNodes", "172.16.46.58", "172.16.101.30")
}

func (jh *jolokiaHandler) returns2MovingNodes() {
	jh.returnNodesForMbean("MovingNodes", "172.16.46.58", "172.16.101.30")
}

func (jh *jolokiaHandler) returnsLiveNodes(nodeIPs ...string) {
	jh.returnNodesForMbean("LiveNodes", nodeIPs...)
}

func (jh *jolokiaHandler) returnsUnreachableNodes(nodeIPs ...string) {
	jh.returnNodesForMbean("UnreachableNodes", nodeIPs...)
}

func (jh *jolokiaHandler) returnsNoLiveNodes() {
	jh.returnsNoDataForMbean("LiveNodes")
}

func (jh *jolokiaHandler) returnsNoUnreachableNodes() {
	jh.returnsNoDataForMbean("UnreachableNodes")
}

func (jh *jolokiaHandler) returnsNoJoiningNodes() {
	jh.returnsNoDataForMbean("JoiningNodes")
}

func (jh *jolokiaHandler) returnsNoLeavingNodes() {
	jh.returnsNoDataForMbean("LeavingNodes")
}

func (jh *jolokiaHandler) returnsNoMovingNodes() {
	jh.returnsNoDataForMbean("MovingNodes")
}

func (jh *jolokiaHandler) returnsNoDataForMbean(mbean string) {
	jh.responsePrimers[mbean] = jolokiaResponsePrimer{
		response: fmt.Sprintf(`{
  "request": {
	"mbean": "org.apache.cassandra.db:type=StorageService",
	"attribute": "%s",
	"type": "read"
  },
  "value": [],
  "timestamp": 1524056270,
  "status": 200
}`, mbean),
		statusCode: 200,
	}
}

func (jh *jolokiaHandler) returnsANonJSONResponse() {
	jh.responsePrimers["LiveNodes"] = jolokiaResponsePrimer{
		response:   `some non json error response`,
		statusCode: 200,
	}
}

func (jh *jolokiaHandler) returnsRackForNode(rack string, nodeIP string) {
	jh.responsePrimers[fmt.Sprintf("getRack/%s", nodeIP)] = jolokiaResponsePrimer{
		response: fmt.Sprintf(`{
  "request": {
	"mbean": "org.apache.cassandra.db:type=EndpointSnitchInfo",
	"arguments": [
	  "%s"
	],
	"type": "exec",
	"operation": "getRack"
  },
  "value": "%s",
  "timestamp": 1525190806,
  "status": 200
}`, nodeIP, rack),
		statusCode: 200,
	}
}

func (jh *jolokiaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for requestPathSubstring, primedResponse := range jh.responsePrimers {
		if strings.Contains(r.URL.Path, requestPathSubstring) {
			w.WriteHeader(primedResponse.statusCode)
			_, err := w.Write([]byte(primedResponse.response))
			Expect(err).NotTo(HaveOccurred())
			return
		}
	}

	w.WriteHeader(404)
	_, err := w.Write([]byte("Not Found"))
	Expect(err).NotTo(HaveOccurred())
}

func aCluster(clusterName, namespace string) *cluster.Cluster {
	clusterDef := apis.ACassandra().WithDefaults().WithName(clusterName).WithNamespace(namespace).Build()
	return cluster.New(clusterDef)
}
