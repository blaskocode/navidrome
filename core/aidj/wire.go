package aidj

import "github.com/google/wire"

var Set = wire.NewSet(
	NewSessionManager,
	NewEnrichmentService,
)
