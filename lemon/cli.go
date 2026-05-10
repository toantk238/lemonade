package lemon

import (
	"io"
	"time"
)

type CommandType int

// Commands
const (
	OPEN CommandType = iota + 1
	COPY
	PASTE
	SERVER
)

const (
	Success        = 0
	FlagParseError = iota + 10
	RPCError
	Help
)

type CommandStyle int

const (
	ALIAS CommandStyle = iota + 1
	SUBCOMMAND
)

type CLI struct {
	In       io.Reader
	Out, Err io.Writer

	Type       CommandType
	DataSource string
	RawData    []byte // set when stdin contains binary file data
	IsFileData bool   // true when RawData is populated from stdin

	// options
	Port           int
	Allow          string
	Host           string
	TransLoopback  bool
	TransLocalfile bool
	LineEnding     string
	LogLevel       int
	Timeout        time.Duration

	Help bool

	NoFallbackMessages bool
	TrimNewline        bool
	StdinIsTTY         bool
	ImageCacheTTL      time.Duration
}
