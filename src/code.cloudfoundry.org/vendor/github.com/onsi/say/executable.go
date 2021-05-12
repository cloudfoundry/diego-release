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
		Fprintln(os.Stderr, 0, Red(context))
		Fprintln(os.Stderr, 0, Red(err.Error()))
		os.Exit(1)
	}
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

	Println(0, Red("Unkown command: %s", os.Args[1]))
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
		Fprintln(os.Stderr, 0, Red("Unkown command: %s", os.Args[2]))
	}

	Fprintln(os.Stderr, 0, Green(executable.Name))
	Fprintln(os.Stderr, 0, executable.Description)

	Fprintln(os.Stderr, 0, "%s", Cyan("Help and Autocompletion"))
	Fprintln(os.Stderr, 0, strings.Repeat("-", len("Help and Autocompletion")))
	Fprintln(os.Stderr, 1, "%s %s", Green("help"), LightGray("[command] - Show this help, or detailed help for the passed in command"))
	Fprintln(os.Stderr, 1, "%s %s", Green("completions"), LightGray("Generate BASH Completions for %s", executable.Name))
	Fprintln(os.Stderr, 0, "")

	for _, commandGroup := range executable.CommandGroups {
		usageForCommandGroup(commandGroup, false)
		Println(0, "")
	}

}

func usageForCommandGroup(commandGroup CommandGroup, includeFlags bool) {
	Fprintln(os.Stderr, 0, "%s - %s", Cyan(commandGroup.Name), LightGray(commandGroup.Description))
	Fprintln(os.Stderr, 0, strings.Repeat("-", len(commandGroup.Name)+3+len(commandGroup.Description)))
	for _, command := range commandGroup.Commands {
		usageForCommand(1, command, includeFlags)
	}
}

func usageForCommand(indentation int, command Command, includeFlags bool) {
	Fprintln(os.Stderr, indentation, "%s %s", Green(command.Name), LightGray(command.Description))
	if includeFlags {
		buffer := &bytes.Buffer{}
		command.FlagSet.SetOutput(buffer)
		command.FlagSet.PrintDefaults()
		Fprintln(os.Stderr, indentation, buffer.String())
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

	Println(0, out)
}
