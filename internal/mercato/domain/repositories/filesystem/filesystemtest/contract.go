package filesystemtest

type MockFilesystem struct {
	ReadFileFn       func(path string) ([]byte, error)
	WriteFileFn      func(path string, content []byte) error
	DeleteFileFn     func(path string) error
	FileExistsFn     func(path string) bool
	DirExistsFn      func(path string) bool
	MkdirAllFn       func(path string) error
	RemoveAllFn      func(path string) error
	MD5ChecksumFn    func(content []byte) string
	TempFileFn       func(name string, content []byte) (string, error)
	RemoveTempFileFn func(path string) error
}

func (m *MockFilesystem) ReadFile(path string) ([]byte, error) {
	if m.ReadFileFn != nil {
		return m.ReadFileFn(path)
	}
	return nil, nil
}

func (m *MockFilesystem) WriteFile(path string, content []byte) error {
	if m.WriteFileFn != nil {
		return m.WriteFileFn(path, content)
	}
	return nil
}

func (m *MockFilesystem) DeleteFile(path string) error {
	if m.DeleteFileFn != nil {
		return m.DeleteFileFn(path)
	}
	return nil
}

func (m *MockFilesystem) FileExists(path string) bool {
	if m.FileExistsFn != nil {
		return m.FileExistsFn(path)
	}
	return false
}

func (m *MockFilesystem) DirExists(path string) bool {
	if m.DirExistsFn != nil {
		return m.DirExistsFn(path)
	}
	return false
}

func (m *MockFilesystem) MkdirAll(path string) error {
	if m.MkdirAllFn != nil {
		return m.MkdirAllFn(path)
	}
	return nil
}

func (m *MockFilesystem) RemoveAll(path string) error {
	if m.RemoveAllFn != nil {
		return m.RemoveAllFn(path)
	}
	return nil
}

func (m *MockFilesystem) MD5Checksum(content []byte) string {
	if m.MD5ChecksumFn != nil {
		return m.MD5ChecksumFn(content)
	}
	return ""
}

func (m *MockFilesystem) TempFile(name string, content []byte) (string, error) {
	if m.TempFileFn != nil {
		return m.TempFileFn(name, content)
	}
	return "", nil
}

func (m *MockFilesystem) RemoveTempFile(path string) error {
	if m.RemoveTempFileFn != nil {
		return m.RemoveTempFileFn(path)
	}
	return nil
}
