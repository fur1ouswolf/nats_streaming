package app

type App interface {
	Start() error
	Stop() error
}
