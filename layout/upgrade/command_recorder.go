package upgrade

import "strings"

// commandExecution is one subprocess the upgrader actually attempted. The
// error is kept so a failed command is not mistaken for a successful one.
type commandExecution struct {
	command string
	err     error
}

// commandRecorder collects the subprocesses the upgrade domain actually runs.
// Reporting is append-only so an execution is recorded even when it fails.
type commandRecorder struct {
	executions []commandExecution
}

func (r *commandRecorder) record(command string, err error) {
	if r == nil {
		return
	}
	r.executions = append(r.executions, commandExecution{command: command, err: err})
}

// commandsRun renders the attempted commands. A failed execution carries a
// "(failed)" suffix so the record remains truthful.
func (r *commandRecorder) commandsRun() []string {
	if r == nil || len(r.executions) == 0 {
		return nil
	}
	commands := make([]string, 0, len(r.executions))
	for _, execution := range r.executions {
		command := execution.command
		if execution.err != nil {
			command += " (failed)"
		}
		commands = append(commands, command)
	}
	return commands
}

func joinCommand(name string, args []string) string {
	return strings.Join(append([]string{name}, args...), " ")
}
