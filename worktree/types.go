package main

type gitStatus struct {
	Unpushed  int
	Modified  int
	DiffLines []string // git diff --stat output lines
}

type moduleInfo struct {
	Name        string
	Path        string
	Description string
	Latest      string
	Ahead       int
	GitBranch   string
	Git         string
	GitMsgs     []string
	Uses        []string
	UsedBy      []string
}

type requireInfo struct {
	path    string
	version string
}
