package param

type OpenParam struct {
	URI           string
	TransLoopback bool
}

type FileEntry struct {
	Name  string
	Bytes []byte
}

type CopyFileParam struct {
	Files    []FileEntry
	ClientID string
}

type PasteFileParam struct {
	ClientID string
}

type PasteFileResult struct {
	SameClient bool
	Files      []FileEntry
}
