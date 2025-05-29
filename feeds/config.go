package feeds

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
	// TempDir is a temporary directory, set explicitly.
	TempDir string
	// EndpointURL for OAI-PMH (currently unused)
	EndpointURL string
	// Date to harvest the data for. We may remove this, since we want to have
	// interruptable streams and backfill, in the best case, automatically.
	Date time.Time
	// MaxRetries is a generic retry count.
	MaxRetries int
	// Timeout is a generic operation timeout.
	Timeout time.Duration
	// CrossrefApiEmail is an email address sent with every request, as suggested by the crossref rest API.
	CrossrefApiEmail string
	// CrossrefUserAgent is the user agent sent to the crossref API.
	CrossrefUserAgent string
	// CrossrefFeedPrefix is a prefix for each harvested file, to distinguish different runs.
	CrossrefFeedPrefix string
	// CrossrefApiFilter is the search criteria for the crossref API.
	CrossrefApiFilter string
	// RcloneTransfers is passed to rclone, for openalex.
	RcloneTransfers int
	// RcloneCheckers is passed to rclone, for openalex.
	RcloneCheckers int
	// DataciteSyncStart, date string, start date of harvest.
	DataciteSyncStart string
}
