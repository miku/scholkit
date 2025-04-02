package config

import "time"

// Config for feeds, TODO(martin): move to config file and environment
// variables; also consider breaking up the config into sections.
type Config struct {
	// DataDir is the generic data dir for all scholkit tools.
	DataDir string
	// FeedDir is the directory specifically for raw data feeds only. Can be
	// anything, but recommended to be a subdirectory of the DataDir.
	FeedDir string
	// SnapshotDir is where all the snapshots live
	SnapshotDir string
	// Source is the name of the source to process.
	Source string
	// EndpointURL for OAI-PMH (not used currently)
	EndpointURL        string
	Date               time.Time
	MaxRetries         int
	Timeout            time.Duration
	CrossrefApiEmail   string
	CrossrefUserAgent  string
	CrossrefFeedPrefix string
	CrossrefApiFilter  string
	RcloneTransfers    int
	RcloneCheckers     int
	DataciteSyncStart  string
}
