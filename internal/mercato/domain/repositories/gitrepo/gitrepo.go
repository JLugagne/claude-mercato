package gitrepo

import "github.com/JLugagne/claude-mercato/internal/mercato/domain"

// MarketFile holds a file's content and version, as read from a market repo in a single pass.
type MarketFile struct {
	Path    string
	Content []byte
	Version domain.MctVersion
}

type GitRepo interface {
	Clone(url, clonePath string) error
	Fetch(clonePath, branch string) (newHeadSHA string, err error)
	DiffSinceCommit(clonePath, branch, oldSHA string) ([]domain.FileDiff, error)
	ReadFileAtRef(clonePath, branch, filePath, commitSHA string) ([]byte, error)
	FileVersion(clonePath, filePath string) (domain.MctVersion, error)
	RemoteHEAD(clonePath, branch string) (string, error)
	ListFiles(clonePath, branch string) ([]string, error)
	IsValidRepo(clonePath string) bool
	ValidateRemote(url string) error
	ReadGlobalDifftool() (string, error)

	// ReadMarketFiles opens the repo once and returns all matching files with
	// their content and last-modified version in a single pass.
	ReadMarketFiles(clonePath, branch string) ([]MarketFile, error)
}
