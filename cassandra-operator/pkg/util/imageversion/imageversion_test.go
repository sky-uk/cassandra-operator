package imageversion

import (
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestImageVersion(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Cluster Suite", test.CreateParallelReporters("imageversion"))
}

var _ = Describe("extracting properties from image:version string", func() {
	Describe("extracting version", func() {
		It("should return a correctly specified version on the image", func() {
			// given
			imageAndVersion := ptr.String("cassandra:9.99")

			// when
			version := Version(imageAndVersion)

			// then
			Expect(version).To(Equal("9.99"))
		})

		It("should return an empty version when no image has been specified", func() {
			// given
			var imageAndVersion *string

			// when
			version := Version(imageAndVersion)

			// then
			Expect(version).To(Equal(""))
		})

		It("should return an empty version when the version separator is the last character of the image name", func() {
			// given
			imageAndVersion := ptr.String("cassandra:")

			// when
			version := Version(imageAndVersion)

			// then
			Expect(version).To(Equal(""))
		})

		It("should return an empty version when no version is specified in the image name", func() {
			// given
			imageAndVersion := ptr.String("cassandra")

			// when
			version := Version(imageAndVersion)

			// then
			Expect(version).To(Equal(""))
		})
	})

	Describe("extracting repository path", func() {
		It("should return a correctly specified name for the image", func() {
			// given
			imageAndVersion := ptr.String("r/cassandra:1.2.3")

			// when
			repositoryPath := RepositoryPath(imageAndVersion)

			// then
			Expect(repositoryPath).To(Equal("r"))
		})

		It("should return an empty string when no image has been specified", func() {
			// given
			var imageAndVersion *string

			// when
			repositoryPath := RepositoryPath(imageAndVersion)

			// then
			Expect(repositoryPath).To(Equal(""))
		})

		It("should return an empty string when there is no repository path", func() {
			// given
			imageAndVersion := ptr.String("image:1.2.3")

			// when
			image := RepositoryPath(imageAndVersion)

			// then
			Expect(image).To(Equal(""))
		})

		It("should return an empty string when the repository path is empty", func() {
			// given
			imageAndVersion := ptr.String("/cassandra")

			// when
			repositoryPath := RepositoryPath(imageAndVersion)

			// then
			Expect(repositoryPath).To(Equal(""))
		})
	})
})
