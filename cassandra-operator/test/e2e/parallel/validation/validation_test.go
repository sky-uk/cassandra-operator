package validation

import (
	"fmt"
	"testing"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	. "github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "E2E Suite (Validation Tests)", test.CreateParallelReporters("e2e_validation"))
}

var _ = Context("Cassandra resource validation", func() {
	It("should fail with an incomplete cassandra spec", func() {
		command, output, err := Kubectl(Namespace, "apply", "-f", "testdata/incomplete-spec.yaml")
		Expect(err).To(HaveOccurred(), fmt.Sprintf("Command was: %v \nOutput was %v", command, string(output)))
		Expect(string(output)).To(ContainSubstring(`spec.racks in body is required`))
	})

	It("should fail with an invalid field in a cassandra spec", func() {
		command, output, err := Kubectl(Namespace, "apply", "-f", "testdata/invalid-value-spec.yaml")
		Expect(err).To(HaveOccurred(), fmt.Sprintf("Command was: %v \nOutput was %v", command, string(output)))
		Expect(string(output)).To(ContainSubstring(`spec.racks.replicas in body must be of type integer: "string"`))
	})

	It("should fail with an invalid env variable from config map in a cassandra spec", func() {
		command, output, err := Kubectl(Namespace, "apply", "-f", "testdata/invalid-env-fromconfig-spec.yaml")
		Expect(err).To(HaveOccurred(), fmt.Sprintf("Command was: %v \nOutput was %v", command, string(output)))
		Expect(string(output)).To(ContainSubstring(`spec.pod.env.valueFrom.secretKeyRef in body is required`))
	})

	It("should fail with an invalid env variable from field ref map in a cassandra spec", func() {
		command, output, err := Kubectl(Namespace, "apply", "-f", "testdata/invalid-env-fromfield-spec.yaml")
		Expect(err).To(HaveOccurred(), fmt.Sprintf("Command was: %v \nOutput was %v", command, string(output)))
		Expect(string(output)).To(ContainSubstring(`spec.pod.env.valueFrom.secretKeyRef in body is required`))
	})
})
