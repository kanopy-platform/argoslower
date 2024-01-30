package iplister

import (
	"fmt"
	"sync"
	"time"
)

type CachedIPLister struct {
	lister       *IPLister
	lastSync     time.Time
	syncInterval time.Duration
	lock         *sync.RWMutex
	ips          []string
}

func NewCachedIPLister(lister *IPLister) *CachedIPLister {
	out := &CachedIPLister{
		lister:       lister,
		syncInterval: 5 * time.Minute,
		lock:         &sync.RWMutex{},
	}
	_ = out.setIPs()
	return out
}

func (i *CachedIPLister) GetIPs() ([]string, error) {
	var out []string
	var err error
	if i.lister == nil {
		return out, fmt.Errorf("No lister configured")
	}
	syncNow := i.needSync()

	if syncNow {
		err = i.setIPs()
	}

	out = i.getIPs()
	return out, err
}

func (i *CachedIPLister) needSync() bool {
	i.lock.Lock()
	defer i.lock.Unlock()
	if i.lastSync.IsZero() || time.Since(i.lastSync) >= i.syncInterval {
		return true
	}
	return false
}
func (i *CachedIPLister) getIPs() []string {
	i.lock.RLock()
	defer i.lock.RUnlock()
	out := make([]string, len(i.ips))
	copy(out, i.ips)
	return out
}

func (i *CachedIPLister) setIPs() error {
	i.lock.Lock()
	defer i.lock.Unlock()
	// always update the sync time to keep from overwhelming the reader target
	i.lastSync = time.Now()
	ipList, err := i.lister.GetIPs()
	if err != nil {
		return err
	}

	i.ips = ipList
	return nil
}
