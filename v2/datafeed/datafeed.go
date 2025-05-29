package datafeed

type Source interface {
	Name() string
	Update() error
	Snapshot() error
}

type Updater interface {
	Update() error
}

type Snapshotter interface {
	Snapshot() error
}

// example

type Crossref struct {
	Dir string
}

type DataCite struct {
	Dir string
}
