package banter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type File struct {
	Data     []byte
	Filename string
}

func NewFileFromPath(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &File{Data: data, Filename: filepath.Base(path)}, nil
}

func NewFileFromBytes(data []byte, filename string) (*File, error) {
	if filename == "" {
		return nil, fmt.Errorf("NewFileFromBytes requires filename")
	}
	buf := make([]byte, len(data))
	copy(buf, data)
	return &File{Data: buf, Filename: filename}, nil
}

func NewFileFromReader(r io.Reader, filename string) (*File, error) {
	if filename == "" {
		return nil, fmt.Errorf("NewFileFromReader requires filename")
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &File{Data: data, Filename: filename}, nil
}

func (f *File) String() string {
	return fmt.Sprintf("File(%q, %d bytes)", f.Filename, len(f.Data))
}