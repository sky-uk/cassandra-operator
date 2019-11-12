package hash

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	"k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestConfigMapHash(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "ConfigMap Hash Suite", test.CreateParallelReporters("config_map_hash"))
}

var _ = Describe("config map hash", func() {
	Describe("the hash", func() {
		It("should be empty string for nil config map", func() {
			result := ConfigMapHash(nil)
			Expect(result).To(Equal(""))
		})

		It("should be returned for valid config map", func() {
			result := ConfigMapHash(configMap().Build())
			Expect(result).To(Equal("047a11c926e910f6dba083c9e9f67756eff86e10653279666df110fc2b681a68"))
		})

		It("should be different for two config maps with the same data but different names", func() {
			result1 := ConfigMapHash(configMap().WithName("Test1").Build())
			result2 := ConfigMapHash(configMap().WithName("Test2").Build())
			Expect(result1).To(Not(Equal(result2)))
		})

		It("should be different for two config maps with the same name but different data", func() {
			result1 := ConfigMapHash(configMap().WithDataValue("test-config-value1").Build())
			result2 := ConfigMapHash(configMap().WithDataValue("test-config-value2").Build())
			Expect(result1).To(Not(Equal(result2)))
		})

		It("should be different for a config maps with multiple data items", func() {
			result1 := ConfigMapHash(configMap().WithDataItemCount(1).Build())
			result2 := ConfigMapHash(configMap().WithDataItemCount(2).Build())
			Expect(result1).To(Not(Equal(result2)))
		})

		It("should be returned for valid config map for valid config map with binary data", func() {
			result := ConfigMapHash(configMap().WithDataItemCount(1).Build())
			Expect(result).To(Equal("047a11c926e910f6dba083c9e9f67756eff86e10653279666df110fc2b681a68"))
		})

		It("should be different for two config maps with the same name but different binary data", func() {
			result1 := ConfigMapHash(configMap().WithBinaryItemCount(1).Build())
			result2 := ConfigMapHash(configMap().WithBinaryItemCount(2).Build())
			Expect(result1).To(Not(Equal(result2)))
		})

		It("should be returned for valid config map for with only binary data", func() {
			result := ConfigMapHash(configMap().WithDataItemCount(0).Build())
			Expect(result).To(Equal("0e0f8712b598ccc24b650c56fd1162943362f3a89af2af6dd6d5f53101c75833"))
		})

		It("should return the same value for the same config map when hashed multiple times", func() {
			configMap := configMap().Build()
			result1 := ConfigMapHash(configMap)
			result2 := ConfigMapHash(configMap)
			Expect(result1).To(Equal(result2))
		})
	})
})

type configMapBuilder struct {
	name            string
	dataValue       string
	dataItemCount   uint
	binaryItemCount uint
}

func configMap() *configMapBuilder {
	return &configMapBuilder{
		name:            "Test",
		dataItemCount:   1,
		binaryItemCount: 0,
		dataValue:       "test-config-value",
	}
}

func (b *configMapBuilder) WithName(name string) *configMapBuilder {
	b.name = name
	return b
}

func (b *configMapBuilder) WithDataValue(dataValue string) *configMapBuilder {
	b.dataValue = dataValue
	return b
}

func (b *configMapBuilder) WithDataItemCount(dataItemCount uint) *configMapBuilder {
	b.dataItemCount = dataItemCount
	return b
}

func (b *configMapBuilder) WithBinaryItemCount(binaryItemCount uint) *configMapBuilder {
	b.binaryItemCount = binaryItemCount
	return b
}

func (b *configMapBuilder) Build() *v1.ConfigMap {
	var dataItems map[string]string
	if b.dataItemCount > 0 {
		dataItems = map[string]string{}
		for i := uint(1); i <= b.dataItemCount; i++ {
			dataItems[fmt.Sprintf("item-%d", i)] = b.dataValue
		}
	}

	var binaryData map[string][]byte
	if b.binaryItemCount > 0 {
		binaryData = map[string][]byte{}
		for i := uint(1); i <= b.binaryItemCount; i++ {
			binaryData[fmt.Sprintf("binary-item-%d", i)] = []byte("TEST")
		}
	}

	return &v1.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name: b.name,
		},
		Data:       dataItems,
		BinaryData: binaryData,
	}
}
