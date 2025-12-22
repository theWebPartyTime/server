package postgres

import (
	"context"
	"server/internal/models"
	"server/internal/repository"

	"gorm.io/gorm"
)

type postgresScriptsRepo struct {
	db *gorm.DB
}

func NewPostgresScriptsRepository(db *gorm.DB) repository.ScriptsRepository {
	return &postgresScriptsRepo{db: db}
}

func (r *postgresScriptsRepo) GetPublicScripts(ctx context.Context, limit int, offset int, search string) ([]*models.Script, error) {
	var scripts []*models.Script
	query := r.db.WithContext(ctx).Where("public = true")

	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("title ILIKE ? OR description ILIKE ?", searchPattern, searchPattern)
	}

	err := query.Limit(limit).Offset(offset).Find(&scripts).Error
	if err != nil {
		return nil, err
	}

	return scripts, nil
}

func (r *postgresScriptsRepo) GetUserScripts(ctx context.Context, userId int, limit int, offset int, search string) ([]*models.Script, error) {
	var scripts []*models.Script
	query := r.db.WithContext(ctx).Where("creator_id = ?", userId)

	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("title ILIKE ? OR description ILIKE ?", searchPattern, searchPattern)
	}

	err := query.Limit(limit).Offset(offset).Find(&scripts).Error
	if err != nil {
		return nil, err
	}

	return scripts, nil
}

func (r *postgresScriptsRepo) GetScriptByHash(ctx context.Context, scriptHash string) (*models.Script, error) {
	var script models.Script
	err := r.db.WithContext(ctx).Where("script_hash = ?", scriptHash).First(&script).Error
	if err != nil {
		return nil, err
	}
	return &script, nil
}

func (r *postgresScriptsRepo) CreateScript(ctx context.Context, script models.Script) error {
	return r.db.WithContext(ctx).Create(script).Error
}

func (r *postgresScriptsRepo) UpdateScript(ctx context.Context, script models.Script) error {
	return r.db.WithContext(ctx).Save(script).Error
}

func (r *postgresScriptsRepo) DeleteScript(ctx context.Context, scriptHash string) error {
	return r.db.WithContext(ctx).Where("script_hash = ?", scriptHash).Delete(&models.Script{}).Error
}
