package file

import (
	"context"
	"io"
	"os"
)

type File struct {
	filename string
}

func New(filename string) *File {
	return &File{
		filename: filename,
	}
}

func (f *File) Data(ctx context.Context) (io.ReadCloser, error) {
	return os.Open(f.filename)
}
