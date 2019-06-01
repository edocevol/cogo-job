package gjobs

type BaseJob interface {
	Run(name string, args ...interface{})
}
