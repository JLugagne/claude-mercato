package service

import "github.com/JLugagne/claude-mercato/internal/mercato/domain"

type ConfigCommands interface {
	SetConfigField(key, value string) error
	GetConfig() (domain.Config, error)
}
