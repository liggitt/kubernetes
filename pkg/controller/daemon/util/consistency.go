package util

import (
	"sync"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/resourceversion"
	"k8s.io/klog/v2"
)

type ConsistencyStore struct {
	reads *resourceVersions

	// writesLock guards reads/additions/deletions to the writes map.
	// individual records are responsible for managing their own thread safety.
	writesLock sync.RWMutex
	// writes is a map of owner -> ownerRecord
	writes map[types.NamespacedName]*ownerRecord
}

func NewConsistencyStore() *ConsistencyStore {
	return &ConsistencyStore{
		writes: map[types.NamespacedName]*ownerRecord{},
		reads:  newResourceVersions(),
	}
}

// getWrittenRecord returns the record for the given owner, or nil if no record exists.
func (c *ConsistencyStore) getWrittenRecord(owner types.NamespacedName) *ownerRecord {
	c.writesLock.RLock()
	defer c.writesLock.RUnlock()
	return c.writes[owner]
}

// ensureWrittenRecord returns a ownerRecord for the given owner and ownerUID.
// If there is no current record, one is created.
// If there is a current record with a different ownerUID, it is replaced with an empty record for the specified ownerUID.
func (c *ConsistencyStore) ensureWrittenRecord(owner types.NamespacedName, ownerUID types.UID) *ownerRecord {
	// fast path, already exists
	if record := c.getWrittenRecord(owner); record != nil && record.ownerUID == ownerUID {
		return record
	}

	// slow path, init
	c.writesLock.Lock()
	defer c.writesLock.Unlock()
	// check again after write lock
	if record := c.writes[owner]; record != nil && record.ownerUID == ownerUID {
		return record
	}
	// initialize to the given uid
	record := newOwnerRecord(ownerUID)
	c.writes[owner] = record
	return record
}

func (c *ConsistencyStore) WroteAt(owner types.NamespacedName, ownerUID types.UID, resource schema.GroupResource, rv string) {
	c.ensureWrittenRecord(owner, ownerUID).WroteAt(resource, rv)
}

// ReadAt records a read for the given resource at the given resource version
func (c *ConsistencyStore) ReadAt(resource schema.GroupResource, rv string) {
	c.reads.getOrCreate(resource, rv).RaiseTo(rv)
}

// Clear deletes the record for owner if it exists and matches the specified ownerUID (or the specified ownerUID is empty)
func (c *ConsistencyStore) Clear(owner types.NamespacedName, ownerUID types.UID) {
	// all deleted daemonsets are expected to have a record, not worth checking the fast path for missing records
	c.writesLock.Lock()
	defer c.writesLock.Unlock()
	if record := c.writes[owner]; record != nil && (len(ownerUID) == 0 || record.ownerUID == ownerUID) {
		delete(c.writes, owner)
	}
}

func (c *ConsistencyStore) IsReady(owner types.NamespacedName) bool {
	record := c.getWrittenRecord(owner)
	return record == nil || record.IsReady(c)
}

type ownerRecord struct {
	// ownerUID must not be mutated after creation
	ownerUID types.UID
	versions *resourceVersions
}

func newOwnerRecord(ownerUID types.UID) *ownerRecord {
	return &ownerRecord{ownerUID: ownerUID, versions: newResourceVersions()}
}

func (w *ownerRecord) WroteAt(resource schema.GroupResource, rv string) {
	w.versions.getOrCreate(resource, rv).RaiseTo(rv)
}
func (w *ownerRecord) IsReady(c *ConsistencyStore) bool {
	w.versions.versionsLock.RLock()
	defer w.versions.versionsLock.RUnlock()
	for gk, owner := range w.versions.versions {
		read := c.reads.get(gk)
		if read == nil {
			klog.InfoS("not ready, no read version recorded", "type", gk, "owner", owner, "read", nil)
			return false
		}
		i, err := read.CompareTo(owner)
		if err != nil {
			klog.ErrorS(err, "not ready, error comparing resource versions", "type", gk, "owner", owner, "read", read)
			// comparison errors indicate there's a data problem with resource versions, continue so we don't block syncing
			continue
		}
		if i < 0 {
			// read version is not as new as owner version, not ready
			klog.InfoS("not ready, read version is not as new as owner version", "type", gk, "owner", owner, "read", read)
			return false
		}
	}
	return true
}

type resourceVersions struct {
	// versionsLock guards reads/adds/deletions from the versions map.
	// individual records are responsible for managing their own thread safety.
	versionsLock sync.RWMutex
	versions     map[schema.GroupResource]*highWaterResourceVersion
}

func newResourceVersions() *resourceVersions {
	return &resourceVersions{
		versions: map[schema.GroupResource]*highWaterResourceVersion{},
	}
}

func (r *resourceVersions) get(resource schema.GroupResource) *highWaterResourceVersion {
	r.versionsLock.RLock()
	defer r.versionsLock.RUnlock()
	return r.versions[resource]
}

func (r *resourceVersions) getOrCreate(resource schema.GroupResource, rv string) *highWaterResourceVersion {
	// fast path, already exists
	r.versionsLock.RLock()
	record, ok := r.versions[resource]
	r.versionsLock.RUnlock()
	if !ok {
		// slow path, init
		r.versionsLock.Lock()
		defer r.versionsLock.Unlock()
		record, ok = r.versions[resource]
		if !ok {
			record = newHighWaterResourceVersion(rv)
			r.versions[resource] = record
		}
	}
	return record
}

type highWaterResourceVersion struct {
	version atomic.Pointer[string]
}

func newHighWaterResourceVersion(rv string) *highWaterResourceVersion {
	record := &highWaterResourceVersion{}
	record.version.Store(&rv)
	return record
}

func (h *highWaterResourceVersion) String() string {
	return *h.version.Load()
}

func (h *highWaterResourceVersion) RaiseTo(v string) {
	for {
		old := h.version.Load()
		i, err := resourceversion.CompareResourceVersion(*old, v)
		if err == nil && i >= 0 {
			return
		}
		if h.version.CompareAndSwap(old, &v) {
			return
		}
	}
}
func (h *highWaterResourceVersion) CompareTo(v *highWaterResourceVersion) (int, error) {
	return resourceversion.CompareResourceVersion(*h.version.Load(), *v.version.Load())
}
