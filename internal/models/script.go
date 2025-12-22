package models

import (
	"io"
	"time"
)

type Script struct {
	ID          int       `json:"id"`
	Public      bool      `json:"public"`
	CreatorId   int       `json:"creator_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ScriptHash  string    `json:"script_hash"`
	CoverHash   string    `json:"cover_hash"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Script) TableName() string {
	return "scenarios"
}

type CreateScript struct {
	ScriptFile  io.Reader `json:"script_file"`
	CoverFile   io.Reader `json:"cover_file"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Public      bool      `json:"public"`
	CreatorId   int       `json:"creator_id"`
}

type UpdateScript struct {
	ScriptFile  io.Reader `json:"script_file"`
	CoverFile   io.Reader `json:"cover_file"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Public      bool      `json:"public"`
}
