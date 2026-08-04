package keys

import "github.com/miles-w-3/lobot/internal/command"

type WorkloadLogsCmd int

const (
	WorkloadLogsCmdUp WorkloadLogsCmd = iota
	WorkloadLogsCmdDown
)

func NewWorkloadLogsRegistry() *command.Registry[WorkloadLogsCmd] {
	r := command.NewRegistry[WorkloadLogsCmd]()

	navigation := r.NewUnifiedCommandGroup("Selector Navigation", "navigate choices")
	navigation.Add("up", WorkloadLogsCmdUp)
	navigation.Add("down", WorkloadLogsCmdDown)

	// TODO: Move builds - it would be better to run them from central, automated location
	// Could also build after each command add, but it's wasteful?
	if err := r.Build(); err != nil {
		panic(err)
	}

	return r
}
