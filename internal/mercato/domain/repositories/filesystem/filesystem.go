package filesystem

type Filesystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, content []byte) error
	DeleteFile(path string) error
	FileExists(path string) bool
	DirExists(path string) bool
	MkdirAll(path string) error
	RemoveAll(path string) error
	MD5Checksum(content []byte) string
	TempFile(name string, content []byte) (string, error)
	RemoveTempFile(path string) error
	ListFiles(dir, suffix string) ([]string, error)
}
