package main

import (
	"encoding/json"
	logr "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra"
	"net/http"
)

type statusCheck struct {
}

func newStatusCheck() *statusCheck {
	return &statusCheck{}
}

// Status is the top level structure containing information about this Operator and the cluster it manages
type Status struct {
	// CassandraCrdVersion is the CRD version supported by this Operator
	CassandraCrdVersion string
}

func (s *statusCheck) statusPage(resp http.ResponseWriter, req *http.Request) {
	statusContent := Status{
		cassandra.Version,
	}

	bytes, err := json.Marshal(statusContent)
	if err != nil {
		handleServerErrorf(resp, "Error while generation status page content, %v", err)
	}

	resp.WriteHeader(200)
	resp.Write(bytes)
}

func handleServerErrorf(resp http.ResponseWriter, message string, args ...interface{}) {
	resp.WriteHeader(500)
	resp.Write([]byte("Error while executing check. Please consult logs for details."))
	logr.Errorf(message, args...)
}
