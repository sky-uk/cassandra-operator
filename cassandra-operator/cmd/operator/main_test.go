package main

import (
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCommandLine(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Command Line Suite", test.CreateParallelReporters("command_line"))
}

var _ = Describe("operator command line", func() {
	Describe("--help", func() {
		It("should print available flags", func() {
			output, err := exec.Command("cassandra-operator", "--help").CombinedOutput()
			Expect(err).ToNot(HaveOccurred())
			Expect(string(output)).To(ContainSubstring("-h, --help"))
			Expect(string(output)).To(ContainSubstring("--log-level"))
			Expect(string(output)).To(ContainSubstring("--metric-request-timeout"))
			Expect(string(output)).To(ContainSubstring("--metric-poll-interval"))
			Expect(string(output)).To(ContainSubstring("--controller-sync-period"))
		})
		It("should list the available log levels", func() {
			output, err := exec.Command("cassandra-operator", "--help").CombinedOutput()
			Expect(err).ToNot(HaveOccurred())
			Expect(string(output)).To(MatchRegexp("--log-level string\\s+should be one of: debug, info, warn, error, fatal, panic \\(default \"info\"\\)"))
		})
	})

	Describe("--metric-poll-interval", func() {
		It("should reject a value that is not positive", func() {
			output, err := exec.Command("cassandra-operator", "--metric-poll-interval=-1s").CombinedOutput()
			Expect(err).To(HaveOccurred())
			Expect(string(output)).To(ContainSubstring("invalid metric-poll-interval"))
		})
	})

	Describe("--log-level", func() {
		It("should reject unknown log level", func() {
			output, err := exec.Command("cassandra-operator", "--log-level=debugwrong").CombinedOutput()
			Expect(err).To(HaveOccurred())
			Expect(string(output)).To(ContainSubstring("invalid log-level"))
		})
	})

	Describe("--controller-sync-period", func() {
		It("should reject a value that is negative", func() {
			output, err := exec.Command("cassandra-operator", "--controller-sync-period=-1s").CombinedOutput()
			Expect(err).To(HaveOccurred())
			Expect(string(output)).To(ContainSubstring("invalid controller-sync-period"))
		})
	})
})
