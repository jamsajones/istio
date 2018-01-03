// Copyright 2017 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"sync"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"

	"istio.io/istio/pilot/platform/kube"
)

// Registry is the standard interface for identity registry implementation
type Registry interface {
	Check(string, string) bool
	AddMapping(string, string)
	DeleteMapping(string, string)
}

// IdentityRegistry is a naive registry that maintains a mapping between
// identities (as strings): id1 -> id2, id3 -> id4, etc. The method call
// Check(id1, id2) will succeed only if there is a mapping id1 -> id2 stored
// in this registry.
//
// CA can make authorization decisions based on this registry. By creating a
// mapping id1 -> id2, CA will approve CSRs sent only by services running
// as id1 for identity id2.
type IdentityRegistry struct {
	sync.RWMutex
	Map map[string]string
}

// Check checks whether id1 is mapped to id2
func (reg *IdentityRegistry) Check(id1, id2 string) bool {
	reg.RLock()
	mapped, ok := reg.Map[id1]
	reg.RUnlock()
	if !ok || id2 != mapped {
		glog.Warningf("Identity %q does not exist or is not mapped to %q", id1, id2)
		return false
	}
	return true
}

// AddMapping adds a mapping id1 -> id2
func (reg *IdentityRegistry) AddMapping(id1, id2 string) {
	reg.RLock()
	oldID, ok := reg.Map[id1]
	reg.RUnlock()
	if ok {
		glog.Warningf("Overwriting existing mapping: %q -> %q", id1, oldID)
	}
	reg.Lock()
	reg.Map[id1] = id2
	reg.Unlock()
}

// DeleteMapping attempts to delete mapping id1 -> id2. If id1 is already
// mapped to a different identity, deletion fails
func (reg *IdentityRegistry) DeleteMapping(id1, id2 string) {
	reg.RLock()
	oldID, ok := reg.Map[id1]
	reg.RUnlock()
	if !ok || oldID != id2 {
		glog.Warningf("Could not delete nonexistent mapping: %q -> %q", id1, id2)
		return
	}
	reg.Lock()
	delete(reg.Map, id1)
	reg.Unlock()
}

var (
	// singleton object of identity registry
	reg Registry
)

// GetIdentityRegistry returns the identity registry object
func GetIdentityRegistry() Registry {
	if reg == nil {
		reg = &IdentityRegistry{
			Map: make(map[string]string),
		}
	}
	return reg
}

// K8SServiceAdded is a handler used by k8s service controller to monitor
// new services and to add their service accounts to registry, if exist
func K8SServiceAdded(svc *v1.Service) {
	svcAcct, ok := svc.ObjectMeta.Annotations[kube.KubeServiceAccountsOnVMAnnotation]
	if ok {
		GetIdentityRegistry().AddMapping(svcAcct, svcAcct)
	}
}

// K8SServiceDeleted is a handler used by k8s service controller to monitor
// deleted services and to remove their service accounts from registry
func K8SServiceDeleted(svc *v1.Service) {
	svcAcct, ok := svc.ObjectMeta.Annotations[kube.KubeServiceAccountsOnVMAnnotation]
	if ok {
		GetIdentityRegistry().DeleteMapping(svcAcct, svcAcct)
	}
}

// K8SServiceUpdated is a handler used by k8s service controller to monitor
// service updates and update the registry
func K8SServiceUpdated(oldSvc, newSvc *v1.Service) {
	K8SServiceDeleted(oldSvc)
	K8SServiceAdded(newSvc)
}
