package iplister

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockReader struct {
	count int
	ret   string
	err   error
}

func (m *mockReader) Data(ctx context.Context) (io.ReadCloser, error) {
	m.count++
	reader := strings.NewReader(m.ret)
	return io.NopCloser(reader), m.err
}

type mockDecoder struct {
	count int
	ret   []string
	err   error
}

func (m *mockDecoder) Decode(data io.ReadCloser) ([]string, error) {
	m.count++
	return m.ret, m.err
}

func TestNew(t *testing.T) {
	t.Parallel()

	i := New(&mockReader{}, &mockDecoder{})
	assert.Equal(t, defaultTimeout, i.timeout)
}

func TestWithTimeout(t *testing.T) {
	t.Parallel()

	fakeTimeout := time.Second * 123

	i := New(&mockReader{}, &mockDecoder{}, WithTimeout(fakeTimeout))
	assert.Equal(t, fakeTimeout, i.timeout)
}

func TestGetIPs(t *testing.T) {
	t.Parallel()

	fakeData := "1.2.3.4/32"

	mockReader := &mockReader{
		ret: fakeData,
		err: nil,
	}
	mockDecoder := &mockDecoder{
		ret: []string{fakeData},
		err: nil,
	}

	i := New(mockReader, mockDecoder)

	res, err := i.GetIPs()
	assert.NoError(t, err)

	assert.Len(t, res, 1)
	assert.Equal(t, fakeData, res[0])

	assert.Equal(t, 1, mockReader.count)
	assert.Equal(t, 1, mockDecoder.count)
}

func TestValidateCIDRs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []string
		wantErr bool
	}{
		{
			name:    "valid",
			input:   []string{"1.2.3.4/32", "2001:db8:a0b:12f0::1/32"},
			wantErr: false,
		},
		{
			name:    "missing suffix",
			input:   []string{"1.2.3.4"},
			wantErr: true,
		},
		{
			name:    "invalid",
			input:   []string{"abc"},
			wantErr: true,
		},
	}

	for _, test := range tests {
		err := ValidateCIDRs(test.input)
		assert.Equal(t, test.wantErr, err != nil, test.name)
	}
}

func TestNewCachedIPLister(t *testing.T) {
	t.Parallel()

	il := New(&mockReader{}, &mockDecoder{}, WithTimeout(defaultTimeout))
	ipl := NewCachedIPLister(il)
	assert.Equal(t, defaultTimeout, ipl.lister.timeout)
}

func TestCachedIPListerGetIPs(t *testing.T) {
	t.Parallel()

	fakeData := "1.2.3.4/32"
	decoderData := []string{fakeData}

	mr := &mockReader{
		ret: fakeData,
		err: nil,
	}
	md := &mockDecoder{
		ret: decoderData,
		err: nil,
	}
	il := New(mr, md)
	ipl := NewCachedIPLister(il)

	// Test ip list population at creation time
	ipl.lock.Lock()
	assert.Equal(t, 1, md.count)
	ipl.lock.Unlock()

	// Test ip list is returned from cache
	ips, err := ipl.GetIPs()
	assert.NoError(t, err)
	assert.Contains(t, ips, "1.2.3.4/32")

	ipl.lock.Lock()
	assert.Equal(t, 1, md.count)
	ipl.lock.Unlock()

	// Test ip list is returned from cache
	ipl.lock.Lock()
	md.ret = []string{"2.3.4.5/32"}
	ipl.lock.Unlock()
	ips, err = ipl.GetIPs()
	assert.NoError(t, err)
	assert.NotContains(t, ips, "2.3.4.5/32")

	ipl.lock.Lock()
	assert.Equal(t, 1, md.count)
	ipl.lock.Unlock()

	// Test 5min background sync for stale ip lists
	ipl.lock.Lock()
	ipl.lastSync = ipl.lastSync.Add(-5 * time.Minute)
	ipl.lock.Unlock()

	ips, err = ipl.GetIPs()
	assert.NoError(t, err)
	assert.True(t, contains(ips, "2.3.4.5/32"))

	// Test failed bg sync retains stale ip list
	ipl.lock.Lock()
	md.ret = []string{"3.4.5.6/32"}
	mr.err = fmt.Errorf("slow down")
	ipl.lastSync = ipl.lastSync.Add(-5 * time.Minute)
	rcount := mr.count
	dcount := md.count
	ipl.lock.Unlock()

	ips, err = ipl.GetIPs()
	assert.Error(t, err)
	index := 0
	for ; index <= 100; index++ {
		ips, err = ipl.GetIPs()
		assert.NoError(t, err)
		assert.True(t, !contains(ips, "3.4.5.6/32"))
	}

	ipl.lock.RLock()
	assert.True(t, index >= 100 && mr.count <= (index+rcount), fmt.Sprint(index+rcount), fmt.Sprint(mr.count))
	assert.True(t, index >= 100 && md.count <= (index+dcount), fmt.Sprint(index+dcount), fmt.Sprint(md.count))
	ipl.lock.RUnlock()

	// Test bg sync recovers from intermitant error
	ipl.lock.Lock()
	mr.err = nil
	ipl.lastSync = ipl.lastSync.Add(-5 * time.Minute)
	ipl.lock.Unlock()

	ips, err = ipl.GetIPs()
	assert.NoError(t, err)
	assert.True(t, contains(ips, "3.4.5.6/32"))

}

func BenchmarkCachedIPLister(b *testing.B) {
	fakeData := "1.2.3.4/32"
	decoderData := []string{fakeData}

	mr := &mockReader{
		ret: fakeData,
		err: nil,
	}
	md := &mockDecoder{
		ret: decoderData,
		err: nil,
	}
	il := New(mr, md)
	ipl := NewCachedIPLister(il)

	ec := float64(0)
	for i := 0; i < b.N; i++ {
		_, err := ipl.GetIPs()
		if err != nil {
			ec++
		}

		if i%10 == 0 {
			ipl.lock.Lock()
			ipl.lastSync = ipl.lastSync.Add(-5 * time.Minute)
			ipl.lock.Unlock()
		}

	}

	b.ReportMetric(ec, "Errors")
}

func contains(in []string, val string) bool {
	for _, v := range in {
		if v == val {
			return true
		}
	}
	return false
}
