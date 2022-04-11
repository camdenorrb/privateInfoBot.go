package module

type Module interface {
	IsEnabled() bool
	Enable()
	Disable()
}
