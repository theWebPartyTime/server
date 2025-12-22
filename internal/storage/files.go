package storage

import (
	"context"
	"io"
)

type FilesStorage interface {
	Save(ctx context.Context, hash string, r io.Reader) error
	Delete(ctx context.Context, hash string) error
	Open(ctx context.Context, hash string) (io.ReadCloser, error)
}
