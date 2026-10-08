package model

import "time"

// EntryType identifies a file system object exposed by a provider.
type EntryType uint8

const (
	File EntryType = iota
	Directory
	Symlink
	Other
)

type Entry struct {
	ID      string
	Path    string
	Type    EntryType
	Size    int64
	ModTime time.Time
	Hash    string
	ETag    string
}

func (e Entry) IsFile() bool {
	return e.Type == File
}

func (e Entry) IsDirectory() bool {
	return e.Type == Directory
}
