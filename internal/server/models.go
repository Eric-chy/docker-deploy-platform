package server

type project struct {
	ID             int64
	Name           string
	TeamID         int64
	NodeID         int64
	CurrentYAML    string
	CurrentVersion string
	ImageRepo      string
}

type userTeamRole struct {
	UserID int64
	TeamID int64
	Role   string
}

type node struct {
	ID      int64
	Name    string
	Address string
	Secret  string
}
