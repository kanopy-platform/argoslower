package iplister

import (
	"context"
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
