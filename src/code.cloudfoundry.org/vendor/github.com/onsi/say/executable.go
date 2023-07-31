package say

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
)

type Executable struct {
	Name          string
	Description   string
	CommandGroups []CommandGroup
}

type Command struct {
	Name        string
	Description string
	FlagSet     *flag.FlagSet
	Run         func(args []string)
}

type CommandGroup struct {
	Name        string
	Description string
	Commands    []Command
}

func ExitIfError(context string, err error) {
	if err != nil {
		Fpln(os.Stderr, "{{red}}%s{{/}}", context)
		Fplni(os.Stderr, 1, "{{red}}%s{{/}}", err.Error())
		os.Exit(1)
	}
}

func ExitWith(format string, args ...any) {
	Fpln(os.Stderr, "{{red}}"+format+"{{/}}", args...)
	os.Exit(1)
}

func Invoke(executable Executable) {
	if len(os.Args) == 1 || os.Args[1] == "help" {
		usage(executable)
		os.Exit(1)
	}

	if os.Args[1] == "completions" {
		completions(executable)
		os.Exit(0)
	}

	for _, commandGroup := range executable.CommandGroups {
		for _, command := range commandGroup.Commands {
			if command.Name == os.Args[1] {
				command.FlagSet.Parse(os.Args[2:])
				command.Run(command.FlagSet.Args())
				os.Exit(0)
			}
		}
	}

	Fpln(os.Stderr, "{{red}}Unknown command:%s{{/}}", os.Args[1])
	usage(executable)
}

func usage(executable Executable) {
	if len(os.Args) > 2 {
		matcher := strings.ToLower(os.Args[2])
		for _, commandGroup := range executable.CommandGroups {
			if strings.HasPrefix(strings.ToLower(commandGroup.Name), matcher) {
				usageForCommandGroup(commandGroup, true)
				return
			}

			for _, command := range commandGroup.Commands {
				if strings.HasPrefix(strings.ToLower(command.Name), matcher) {
					usageForCommand(0, command, true)
					return
				}
			}
		}
		Fpln(os.Stderr, "{{red}}Unknown command: %s{{/}}", os.Args[2])
	}

	Fpln(os.Stderr, "{{green}}%s{{/}}", executable.Name)
	Fpln(os.Stderr, executable.Description)

	Fpln(os.Stderr, "{{cyan}}Help and Autocomplete{{/}}")
	Fpln(os.Stderr, strings.Repeat("-", len("Help and Autocomplete")))
	Fplni(os.Stderr, 1, "{{green}}help {{light-gray}}[command] - Show this help, or detailed help for the passed in command{{/}}")
	Fplni(os.Stderr, 1, "{{green}}completions {{light-gray}} - Generate BASH Completions for %s{{/}}", executable.Name)
	Fpln(os.Stderr, "")

	for _, commandGroup := range executable.CommandGroups {
		usageForCommandGroup(commandGroup, false)
		Fpln(os.Stderr, "")
	}

}

func usageForCommandGroup(commandGroup CommandGroup, includeFlags bool) {
	Fpln(os.Stderr, "{{cyan}}%s{{/}} - {{light-gray}}%s{{/}}", commandGroup.Name, commandGroup.Description)
	Fpln(os.Stderr, strings.Repeat("-", len(commandGroup.Name)+3+len(commandGroup.Description)))
	for _, command := range commandGroup.Commands {
		usageForCommand(1, command, includeFlags)
	}
}

func usageForCommand(indentation uint, command Command, includeFlags bool) {
	Fplni(os.Stderr, indentation, "{{green}}%s {{light-gray}}%s{{/}}", command.Name, command.Description)
	if includeFlags {
		buffer := &bytes.Buffer{}
		command.FlagSet.SetOutput(buffer)
		command.FlagSet.PrintDefaults()
		Fplni(os.Stderr, indentation, buffer.String())
	}
}

func completions(executable Executable) {
	availableCommands := []string{}
	for _, commandGroup := range executable.CommandGroups {
		for _, command := range commandGroup.Commands {
			availableCommands = append(availableCommands, command.Name)
		}
	}

	out := fmt.Sprintf(`
function _%s() {
    local cur prev commands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="%s"

    if [[ "${COMP_CWORD}" == "1" ]] ; then
        COMPREPLY=( $(compgen -W "${commands} help completions" -- ${cur}) );
    elif [[ "${prev}" == "help" ]] ; then
        COMPREPLY=( $(compgen -W "${commands}" -- ${cur}) );
    else
        COMPREPLY=( $(compgen -f ${cur}) );
    fi

    return 0
}

complete -F _%s %s
`, executable.Name, strings.Join(availableCommands, " "), executable.Name, executable.Name)

	Pln(out)
}
