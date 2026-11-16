package server

import "regexp"

var projectNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
