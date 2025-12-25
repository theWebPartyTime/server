package local

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/theWebPartyTime/server/internal/storage"
)

type LocalFilesStorage struct {
	baseDir   string
	extension string
}

func NewLocalFilesStorage(baseDir, extension string) storage.FilesStorage {
	return &LocalFilesStorage{baseDir: baseDir, extension: extension}
}

func (s *LocalFilesStorage) Save(ctx context.Context, hash string, r io.Reader) error {
	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return err
	}

	finalPath := filepath.Join(s.baseDir, hash+s.extension)

	if _, err := os.Stat(finalPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	tmpFile, err := os.CreateTemp(s.baseDir, "tmp-*"+s.extension)
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, r); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	if err := os.Rename(tmpPath, finalPath); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	log.Print("file %s created", finalPath)

	return nil
}

func (s *LocalFilesStorage) Delete(ctx context.Context, hash string) error {
	path := filepath.Join(s.baseDir, hash+s.extension)
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

func (s *LocalFilesStorage) Open(ctx context.Context, hash string) (io.ReadCloser, error) {
	path := filepath.Join(s.baseDir, hash+s.extension)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("file not found")
		}
		return nil, err
	}
	return f, nil
}
