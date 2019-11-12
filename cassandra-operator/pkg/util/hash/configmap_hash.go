package hash

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"

	"k8s.io/api/core/v1"
)

// ConfigMapHash returns a hash of the ConfigMap.
func ConfigMapHash(cm *v1.ConfigMap) string {
	if cm == nil {
		log.Warnf("Attempted to hash nil config map!")
		return ""
	}
	encoded := encodeConfigMap(cm)
	return hash(encoded)
}

func encodeConfigMap(cm *v1.ConfigMap) string {

	// json.Marshal sorts the keys in a stable order in the encoding
	m := map[string]interface{}{"name": cm.Name, "data": cm.Data}
	if len(cm.BinaryData) > 0 {
		m["binaryData"] = cm.BinaryData
	}
	data, err := json.Marshal(m)
	if err != nil {
		log.Warnf("Error while marshalling config map: %v", err)
		return ""
	}

	return string(data)
}

func hash(data string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(data)))
}
