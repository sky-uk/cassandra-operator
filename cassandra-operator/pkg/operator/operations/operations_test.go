package operations

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
)

func TestOperations(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Operations Suite", test.CreateParallelReporters("operations"))
}

var _ = Describe("operation composer", func() {
	It("should execute all supplied operations when none of them returns stop=true", func() {
		// given
		sharedState := &sharedState{}
		operations := []Operation{
			&dummyOperation{sharedState, "1", false},
			&dummyOperation{sharedState, "2", false},
			&dummyOperation{sharedState, "3", false},
		}

		// when
		composer := NewOperationComposer()
		stopped, err := composer.Execute(operations)

		// then
		Expect(stopped).To(BeFalse())
		Expect(err).ToNot(HaveOccurred())
		Expect(sharedState.items).To(Equal([]string{"1", "2", "3"}))
	})

	It("should stop executing operations after the one of them returns stop=true", func() {
		// given
		sharedState := &sharedState{}
		operations := []Operation{
			&dummyOperation{sharedState, "1", false},
			&dummyOperation{sharedState, "2", true},
			&dummyOperation{sharedState, "3", false},
		}

		// when
		composer := NewOperationComposer()
		stopped, err := composer.Execute(operations)

		// then
		Expect(stopped).To(BeTrue())
		Expect(err).ToNot(HaveOccurred())
		Expect(sharedState.items).To(Equal([]string{"1", "2"}))
	})

	It("should report all errors produced by the individual operations", func() {
		// given
		operations := []Operation{
			&dummyErrorOperation{fmt.Errorf("err1")},
			&dummyErrorOperation{fmt.Errorf("err2")},
		}

		// when
		composer := NewOperationComposer()
		stopped, err := composer.Execute(operations)

		// then
		Expect(stopped).To(BeFalse())

		Expect(err).To(HaveOccurred())
		Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))

		multiErr := err.(*multierror.Error)
		Expect(multiErr.Errors[0]).To(Equal(fmt.Errorf("error while executing operation dummy error operation: err1")))
		Expect(multiErr.Errors[1]).To(Equal(fmt.Errorf("error while executing operation dummy error operation: err2")))
	})
})

type sharedState struct {
	items []string
}

type dummyOperation struct {
	sharedState *sharedState
	output      string
	stop        bool
}

func (d *dummyOperation) Execute() (bool, error) {
	d.sharedState.items = append(d.sharedState.items, d.output)
	return d.stop, nil
}

func (d *dummyOperation) String() string {
	return "dummy operation"
}

type dummyErrorOperation struct {
	err error
}

func (d *dummyErrorOperation) Execute() (bool, error) {
	return false, d.err
}

func (d *dummyErrorOperation) String() string {
	return "dummy error operation"
}
