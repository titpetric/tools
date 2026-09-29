package config

// Section is a named group of settings, one form heading.
type Section struct {
	Title  string
	Fields []Field
}

// Sections returns the editable settings of the document, in the order the
// form shows them. Every setting the document holds appears exactly once, so
// the form covers the whole file.
func (c *Config) Sections() []Section {
	return []Section{
		{
			Title: "Scan",
			Fields: []Field{
				{
					Title: "Enable Gitignore",
					Key:   "scan.enable_gitignore",
					Bool:  &c.Scan.EnableGitignore,
					Help:  "Skip what a .gitignore excludes",
				},
				{
					Title: "Enable Git Repos",
					Key:   "scan.enable_git_repos",
					Bool:  &c.Scan.EnableGitRepos,
					Help:  "List git repos without a go.mod",
				},
				{
					Title: "Ignore Paths",
					Key:   "scan.ignore_paths",
					List:  &c.Scan.IgnorePaths,
					Help:  "Folder names never descended into",
				},
				{
					Title: "Root Markers",
					Key:   "scan.root_markers",
					List:  &c.Scan.RootMarkers,
					Help:  "Files marking the workspace root",
				},
			},
		},
	}
}
