package say

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func readLine() string {
	in := bufio.NewReader(os.Stdin)
	line, err := in.ReadString('\n')
	if err != nil {
		panic(err)
	}
	return strings.Replace(line, "\n", "", 1)
}

func Ask(f string, args ...any) string {
	Plni(0, f, args...)
	response := readLine()
	return response
}

func AskWithValidation(text string, validation func(string) error) string {
	Pln(text + ":")
	response := readLine()
	err := validation(response)
	if err != nil {
		Plni(0, "{{red}}%s{{/}}", err.Error())
		return AskWithValidation(text, validation)
	}
	return response
}

func AskWithDefault(text string, defaultResponse string) string {
	P("%s [{{green}}%s{{}}]:\n> ", text, defaultResponse)
	response := readLine()
	if response == "" {
		return defaultResponse
	}
	return response
}

func AskForIntegerWithDefault(text string, defaultResponse int) int {
	P("%s [{{green}}%d{{/}}]:\n> ", text, defaultResponse)
	response := readLine()
	if response == "" {
		return defaultResponse
	}
	asInteger, err := strconv.Atoi(response)
	if err != nil {
		Pln("{{red}}That was an invalid response...{{/}}")
		return AskForIntegerWithDefault(text, defaultResponse)
	}

	return asInteger
}

func AskForBoolWithDefault(text string, defaultResponse bool) bool {
	P("%s [{{green}}%t{{/}}]:\n> ", text, defaultResponse)
	response := readLine()
	if response == "true" {
		return true
	}
	if response == "false" {
		return false
	}
	if response == "" {
		return defaultResponse
	}
	Pln("{{red}}That was an invalid response... try 'true' or 'false'{{/}}")
	return AskForBoolWithDefault(text, defaultResponse)
}

func Pick(text string, options []string) string {
	Pln("%s:", text)
	for i, option := range options {
		Plni(1, "[{{green}}%d{{/}}] %s", i, option)
	}
	P("> ")
	response := readLine()
	index, err := strconv.Atoi(response)
	if err != nil {
		Pln("{{red}}That was an invalid selection...{{/}}")
		return Pick(text, options)
	}
	return options[index]
}
