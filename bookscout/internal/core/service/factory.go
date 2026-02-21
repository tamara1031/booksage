package service

import (
	"bookscout/internal/adapters/source"
	"bookscout/internal/config"
	"bookscout/internal/core/domain/ports"
)

func CreateBookSource(cfg *config.Config) ports.BookDataSource {
	switch cfg.BookSourceType {
	case "opds":
		return source.NewOPDSAdapter(cfg.OPDSBaseURL, cfg.OPDSUsername, cfg.OPDSPassword, cfg.MaxBookSizeBytes, cfg.LogLevel)
	default:
		// Default to OPDS
		return source.NewOPDSAdapter(cfg.OPDSBaseURL, cfg.OPDSUsername, cfg.OPDSPassword, cfg.MaxBookSizeBytes, cfg.LogLevel)
	}
}
