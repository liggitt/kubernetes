package util

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/resourceversion"
)

type stalenessInfo struct {
	podRV string
	dsRV  string
}

type ownedInfo struct {
	uid    types.UID
	rvInfo stalenessInfo
}

type ConsistencyStore struct {
	// ownerToRVInfo maps the current resource version of the object if seen
	// previously
	ownerToRVInfo map[types.NamespacedName]ownedInfo
	readStore     stalenessInfo
	mux           sync.RWMutex
}

func NewConsistencyStore() *ConsistencyStore {
	return &ConsistencyStore{
		ownerToRVInfo: make(map[types.NamespacedName]ownedInfo),
		mux:           sync.RWMutex{},
	}
}

func (c *ConsistencyStore) WroteAt(owner types.NamespacedName, ownerUID types.UID, resource schema.GroupVersionKind, rv string) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	cur := c.ownerToRVInfo[owner]
	curRV, err := stalenessRV(cur.rvInfo, resource)
	if err != nil {
		return err
	}
	if curRV != "" {
		cmp, err := resourceversion.CompareResourceVersion(rv, curRV)
		if err != nil {
			return err
		}
		if cmp <= 0 {
			return nil
		}
	}
	err = updateRV(&cur.rvInfo, resource, rv)
	if err != nil {
		return err
	}
	cur.uid = ownerUID
	c.ownerToRVInfo[owner] = cur
	return nil
}

func (c *ConsistencyStore) ReadAt(resource schema.GroupVersionKind, rv string) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	curRV, err := stalenessRV(c.readStore, resource)
	if err != nil {
		return err
	}
	if curRV != "" {
		cmp, err := resourceversion.CompareResourceVersion(rv, curRV)
		if err != nil {
			return err
		}
		if cmp <= 0 {
			return nil
		}
	}
	return updateRV(&c.readStore, resource, rv)
}

func (c *ConsistencyStore) Clear(owner types.NamespacedName, ownerUID types.UID) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	cur, ok := c.ownerToRVInfo[owner]
	if !ok {
		return nil
	}
	if cur.uid == ownerUID {
		delete(c.ownerToRVInfo, owner)
	}
	return nil
}

func (c *ConsistencyStore) IsReady(owner types.NamespacedName) (bool, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()
	wroteObjs, ok := c.ownerToRVInfo[owner]
	if !ok {
		return true, nil
	}
	// Check pod RVs
	if wroteObjs.rvInfo.podRV != "" {
		cmp, err := resourceversion.CompareResourceVersion(c.readStore.podRV, wroteObjs.rvInfo.podRV)
		if err != nil {
			return false, err
		}
		if cmp < 0 {
			return false, nil
		}
	}

	// Check daemonset RVs
	if wroteObjs.rvInfo.dsRV != "" {
		cmp, err := resourceversion.CompareResourceVersion(c.readStore.dsRV, wroteObjs.rvInfo.dsRV)
		if err != nil {
			return false, err
		}
		if cmp < 0 {
			return false, nil
		}
	}

	return true, nil
}

func stalenessRV(cur stalenessInfo, resource schema.GroupVersionKind) (string, error) {
	curRv := ""
	switch resource.Kind {
	case "Pod":
		curRv = cur.podRV
	case "Daemonset":
		curRv = cur.dsRV
	default:
		return "", fmt.Errorf("unsupported type")
	}
	return curRv, nil
}

func updateRV(cur *stalenessInfo, resource schema.GroupVersionKind, rv string) error {
	switch resource.Kind {
	case "Pod":
		cur.podRV = rv
	case "Daemonset":
		cur.dsRV = rv
	default:
		return fmt.Errorf("unsupported type")
	}
	return nil
}
