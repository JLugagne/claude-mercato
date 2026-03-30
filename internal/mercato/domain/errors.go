package domain

import (
	"errors"
	"fmt"
)

type DomainError struct {
	Code    string
	Message string
	Err     error
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

func IsDomainError(err error) bool {
	var de *DomainError
	return errors.As(err, &de)
}

func (e *DomainError) Wrap(err error) *DomainError {
	return &DomainError{Code: e.Code, Message: e.Message, Err: err}
}

var (
	ErrMarketNotFound      = &DomainError{Code: "MARKET_NOT_FOUND", Message: "market not found in configuration"}
	ErrMarketAlreadyExists = &DomainError{Code: "MARKET_ALREADY_EXISTS", Message: "market already exists in configuration"}
	ErrMarketURLExists     = &DomainError{Code: "MARKET_URL_EXISTS", Message: "a market with this URL is already registered"}
	ErrMarketUnreachable   = &DomainError{Code: "MARKET_UNREACHABLE", Message: "could not reach market repository"}
	ErrInvalidMarketName   = &DomainError{Code: "INVALID_MARKET_NAME", Message: "market name must be kebab-case, 2-64 characters"}
)

var (
	ErrEntryNotFound         = &DomainError{Code: "ENTRY_NOT_FOUND", Message: "entry not found"}
	ErrEntryAlreadyInstalled = &DomainError{Code: "ENTRY_ALREADY_INSTALLED", Message: "entry is already installed"}
	ErrEntryNotInstalled     = &DomainError{Code: "ENTRY_NOT_INSTALLED", Message: "entry is not installed"}
	ErrEntryOrphaned         = &DomainError{Code: "ENTRY_ORPHANED", Message: "entry's market has been removed"}
)

var (
	ErrInvalidFrontmatter = &DomainError{Code: "INVALID_FRONTMATTER", Message: "frontmatter missing required fields"}
	ErrInvalidEntryType   = &DomainError{Code: "INVALID_ENTRY_TYPE", Message: "entry type does not match parent directory"}
	ErrMctFieldsInRepo    = &DomainError{Code: "MCT_FIELDS_IN_REPO", Message: "mct_ref/version/market/installed_at must not be in market repo files"}
)

var (
	ErrSkillNotFound     = &DomainError{Code: "SKILL_NOT_FOUND", Message: "required skill not found in market"}
	ErrSkillTypeMismatch = &DomainError{Code: "SKILL_TYPE_MISMATCH", Message: "file exists but type is not 'skill'"}
	ErrPinMismatch       = &DomainError{Code: "PIN_MISMATCH", Message: "pinned SHA does not match current version"}
)

var (
	ErrSyncDirty   = &DomainError{Code: "SYNC_DIRTY", Message: "previous sync did not complete cleanly"}
	ErrCacheStale  = &DomainError{Code: "CACHE_STALE", Message: "market cache is older than stale_after threshold"}
	ErrOfflineMode = &DomainError{Code: "OFFLINE_MODE", Message: "network operation requested in offline mode"}
)

var (
	ErrChecksumMismatch = &DomainError{Code: "CHECKSUM_MISMATCH", Message: "local file has been modified after install"}
)

var (
	ErrConflictRefCollision = &DomainError{Code: "CONFLICT_REF_COLLISION", Message: "same filename in two markets resolves to same local path"}
	ErrConflictDepVersion   = &DomainError{Code: "CONFLICT_DEP_VERSION", Message: "two agents require same skill at incompatible versions"}
	ErrConflictDepDeleted   = &DomainError{Code: "CONFLICT_DEP_DELETED", Message: "a required skill was deleted from its market"}
)

var (
	ErrDifftoolNotFound = &DomainError{Code: "DIFFTOOL_NOT_FOUND", Message: "no difftool available"}
	ErrCloneExists      = &DomainError{Code: "CLONE_EXISTS", Message: "clone directory exists but market is not registered"}
	ErrSSHDisabled      = &DomainError{Code: "SSH_DISABLED", Message: "SSH is disabled. To enable:\n  mct config set ssh_enabled true\nor set MCT_SSH_ENABLED=true"}
)
