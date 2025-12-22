package repository

import (
	"context"
	"server/internal/models"
)

type ScriptsRepository interface {
	GetPublicScripts(ctx context.Context, limit int, offset int, search string) ([]*models.Script, error)
	GetUserScripts(ctx context.Context, userId int, limit int, offset int, search string) ([]*models.Script, error)
	GetScriptByHash(ctx context.Context, scriptHash string) (*models.Script, error)
	CreateScript(ctx context.Context, script models.Script) error
	UpdateScript(ctx context.Context, script models.Script) error
	DeleteScript(ctx context.Context, scriptHash string) error
}
