package iplister

import (
	"context"
	"sync"
	"time"
)

type CachedIPLister struct {
	reader       Reader
	decoder      Decoder
	timeout      time.Duration
	lastSync     time.Time
	syncInterval time.Duration
	lock         *sync.RWMutex
	ips          []string
}

func NewCachedIPLister(reader Reader, decoder Decoder) *CachedIPLister {
	out := &CachedIPLister{
		reader:       reader,
		decoder:      decoder,
		timeout:      defaultTimeout,
		syncInterval: 5 * time.Minute,
		lock:         &sync.RWMutex{},
	}
	_ = out.setIPs()
	return out
}

func (i *CachedIPLister) SetTimeout(t time.Duration) {
	if i == nil {
		return
	}
	i.lock.Lock()
	i.timeout = t
	i.lock.Unlock()
}

func (i *CachedIPLister) GetIPs() ([]string, error) {
	var out []string
	var err error
	syncNow, bgSync := i.needSync()

	if syncNow {
		err = i.setIPs()
	}

	ipl := i.getIPs()
	out = make([]string, len(ipl))
	copy(out, ipl)

	if bgSync {
		go i.setIPs() //nolint:errcheck
	}
	return out, err
}

func (i *CachedIPLister) needSync() (bool, bool) {
	i.lock.Lock()
	defer i.lock.Unlock()
	if i.lastSync.IsZero() {
		return true, false
	}

	if time.Since(i.lastSync) >= i.syncInterval {
		return false, true
	}

	return false, false
}
func (i *CachedIPLister) getIPs() []string {
	i.lock.RLock()
	defer i.lock.RUnlock()
	return i.ips
}

func (i *CachedIPLister) setIPs() error {
	i.lock.Lock()
	defer i.lock.Unlock()
	// always update the sync time to keep from overwhelming the reader target
	i.lastSync = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), i.timeout)
	defer cancel()

	reader, err := i.reader.Data(ctx)
	if err != nil {
		return err
	}
	defer reader.Close()

	ipList, err := i.decoder.Decode(reader)
	if err != nil {
		return err
	}

	if err := ValidateCIDRs(ipList); err != nil {
		return err
	}

	i.ips = ipList
	return nil
}
