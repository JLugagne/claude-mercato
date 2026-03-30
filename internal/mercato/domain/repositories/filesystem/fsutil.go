package filesystem

import "io/fs"

// FileExists returns true if path exists in fsys and is not a directory.
func FileExists(fsys fs.StatFS, path string) bool {
	info, err := fsys.Stat(path)
	return err == nil && !info.IsDir()
}

// DirExists returns true if path exists in fsys and is a directory.
func DirExists(fsys fs.StatFS, path string) bool {
	info, err := fsys.Stat(path)
	return err == nil && info.IsDir()
}
