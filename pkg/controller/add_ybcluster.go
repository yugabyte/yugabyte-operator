package controller

import (
	"github.com/yugabyte/yugabyte-k8s-operator/pkg/controller/ybcluster"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, ybcluster.Add)
}
