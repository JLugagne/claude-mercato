package service

import "github.com/JLugagne/agents-mercato/internal/mercato/domain"

type ConfigCommands interface {
	SetConfigField(key, value string) error
	GetConfig() (domain.Config, error)
}
