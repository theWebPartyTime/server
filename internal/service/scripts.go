package service

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"time"

	"github.com/theWebPartyTime/server/internal/models"
	"github.com/theWebPartyTime/server/internal/repository"
	"github.com/theWebPartyTime/server/internal/storage"
)

type ScriptsService struct {
	scriptsRepo    repository.ScriptsRepository
	scriptsStorage storage.FilesStorage
	imagesStorage  storage.FilesStorage
}

func NewScriptsService(scriptsRepo repository.ScriptsRepository, scriptsStorage storage.FilesStorage, imagesStorage storage.FilesStorage) *ScriptsService {
	return &ScriptsService{scriptsRepo: scriptsRepo, scriptsStorage: scriptsStorage, imagesStorage: imagesStorage}
}

func (s *ScriptsService) UploadScript(ctx context.Context, scriptRequest models.CreateScript) error {
	scriptData, err := io.ReadAll(scriptRequest.ScriptFile)
	if err != nil {
		return err
	}

	scriptHash, err := ComputeHashFromReader(bytes.NewReader(scriptData))
	if err != nil {
		return err
	}

	if err := s.scriptsStorage.Save(ctx, scriptHash, bytes.NewReader(scriptData)); err != nil {
		return err
	}
	var coverHash string
	var coverData []byte

	if scriptRequest.CoverFile != nil {
		coverData, err = io.ReadAll(scriptRequest.CoverFile)
		if err != nil {
			return err
		}
	}

	coverHash, err = ComputeHashFromReader(bytes.NewReader(coverData))
	if err != nil {
		return err
	}

	if err := s.imagesStorage.Save(ctx, coverHash, bytes.NewReader(coverData)); err != nil {
		return err
	}

	var script models.Script
	script.ScriptHash = scriptHash
	script.CoverHash = coverHash
	script.Title = scriptRequest.Title
	script.Description = scriptRequest.Description
	script.CreatorId = scriptRequest.CreatorId
	script.Public = scriptRequest.Public
	script.CreatedAt = time.Now()
	script.UpdatedAt = time.Now()

	err = s.scriptsRepo.CreateScript(ctx, script)
	if err != nil {
		return err
	}
	return nil
}

func (s *ScriptsService) UpdateScript(ctx context.Context, oldScriptHash string, oldCoverHash string, scriptRequest models.UpdateScript) error {
	if scriptRequest.CoverFile != nil {
		err := s.UpdateCover(ctx, oldCoverHash, scriptRequest.CoverFile)
		if err != nil {
			return err
		}
	}
	var newScriptHash string
	if scriptRequest.ScriptFile != nil {

		scriptData, err := io.ReadAll(scriptRequest.ScriptFile)
		if err != nil {
			return err
		}

		newScriptHash, err := ComputeHashFromReader(bytes.NewReader(scriptData))
		if err != nil {
			return err
		}

		if err := s.scriptsStorage.Save(ctx, newScriptHash, bytes.NewReader(scriptData)); err != nil {
			return err
		}

	}

	script, err := s.scriptsRepo.GetScriptByHash(ctx, oldScriptHash)
	if err != nil {
		return err
	}

	if newScriptHash != "" {
		script.ScriptHash = newScriptHash
	}

	if scriptRequest.Title != "" {
		script.Title = scriptRequest.Title
	}
	if scriptRequest.Description != "" {
		script.Description = scriptRequest.Description
	}

	script.Public = scriptRequest.Public
	script.UpdatedAt = time.Now()

	err = s.scriptsRepo.UpdateScript(ctx, *script)
	if err != nil {
		if newScriptHash != "" {
			_ = s.scriptsStorage.Delete(ctx, newScriptHash)
		}
		return err
	}

	if newScriptHash != "" && oldScriptHash != "" {
		_ = s.scriptsStorage.Delete(ctx, oldScriptHash)
	}
	return nil

}

func (s *ScriptsService) DeleteScript(ctx context.Context, scriptHash string) error {
	script, err := s.scriptsRepo.GetScriptByHash(ctx, scriptHash)
	if err != nil {
		return err
	}
	if err := s.scriptsStorage.Delete(ctx, scriptHash); err != nil {
		return err
	}
	if err := s.imagesStorage.Delete(ctx, script.CoverHash); err != nil {
		return err
	}
	err = s.scriptsRepo.DeleteScript(ctx, scriptHash)
	if err != nil {
		return err
	}

	return nil
}

func (s *ScriptsService) UpdateCover(ctx context.Context, scriptHash string, coverFile io.Reader) error {
	coverData, err := io.ReadAll(coverFile)
	if err != nil {
		return err
	}

	coverHash, err := ComputeHashFromReader(bytes.NewReader(coverData))
	if err != nil {
		return err
	}

	script, err := s.scriptsRepo.GetScriptByHash(ctx, scriptHash)
	if err != nil {
		return err
	}

	if err := s.imagesStorage.Save(ctx, coverHash, bytes.NewReader(coverData)); err != nil {
		return err
	}

	oldCoverHash := script.CoverHash

	script.CoverHash = coverHash

	err = s.scriptsRepo.UpdateScript(ctx, *script)
	if err != nil {
		_ = s.imagesStorage.Delete(ctx, coverHash)
		return err

	}

	if oldCoverHash != "" {
		err = s.imagesStorage.Delete(ctx, oldCoverHash)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *ScriptsService) GetUserScripts(ctx context.Context, userId int, limit int, offset int, search string) ([]*models.Script, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	scripts, err := s.scriptsRepo.GetUserScripts(ctx, userId, limit, offset, search)
	if err != nil {
		return nil, err
	}
	return scripts, nil

}

func (s *ScriptsService) GetPublicScripts(ctx context.Context, limit int, offset int, search string) ([]*models.Script, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	scripts, err := s.scriptsRepo.GetPublicScripts(ctx, limit, offset, search)
	if err != nil {
		return nil, err
	}
	return scripts, nil

}

func (s *ScriptsService) GetScriptByHash(ctx context.Context, hash string) (*models.Script, error) {
	return s.scriptsRepo.GetScriptByHash(ctx, hash)
}

func ComputeHashFromReader(r io.Reader) (string, error) {
	hasher := md5.New()
	if _, err := io.Copy(hasher, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
