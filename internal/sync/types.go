package sync

type PRScope string

const (
	PRScopeAuthored PRScope = "authored"
	PRScopeInvolved PRScope = "involved"
	PRScopeBoth     PRScope = "both"
)

type Config struct {
	Repo         string
	Branch       string
	ReadmePath   string
	FullListPath string
	DryRun       bool

	PRFetchLimit   int
	TopContribs    int
	ProjectsToShow int

	SkipOwners    []string
	ProjectsOwner string
	PRsScope      PRScope
	ActorLogin    string
}
