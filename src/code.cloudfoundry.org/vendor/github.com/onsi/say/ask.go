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

func Ask(text string) string {
	Print(0, text+":\n> ")
	response := readLine()
	return response
}

func AskWithValidation(text string, validation func(string) error) string {
	Print(0, text+":\n> ")
	response := readLine()
	err := validation(response)
	if err != nil {
		Println(0, Red(err.Error()))
		return AskWithValidation(text, validation)
	}
	return response
}

func AskWithDefault(text string, defaultResponse string) string {
	Print(0, "%s [%s]:\n> ", text, Green(defaultResponse))
	response := readLine()
	if response == "" {
		return defaultResponse
	}
	return response
}

func AskForIntegerWithDefault(text string, defaultResponse int) int {
	Print(0, "%s [%s]:\n> ", text, Green("%d", defaultResponse))
	response := readLine()
	if response == "" {
		return defaultResponse
	}
	asInteger, err := strconv.Atoi(response)
	if err != nil {
		Println(0, Red("That was an invalid response..."))
		return AskForIntegerWithDefault(text, defaultResponse)
	}

	return asInteger
}

func AskForBoolWithDefault(text string, defaultResponse bool) bool {
	Print(0, "%s [%s]:\n> ", text, Green("%t", defaultResponse))
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
	Println(0, Red("That was an invalid response... try 'true' or 'false'"))
	return AskForBoolWithDefault(text, defaultResponse)
}

func Pick(text string, options []string) string {
	Println(0, "%s:", text)
	for i, option := range options {
		Println(1, "[%s] %s", Green("%d", i), option)
	}
	Print(0, "> ")
	response := readLine()
	index, err := strconv.Atoi(response)
	if err != nil {
		Println(0, Red("That was an invalid selection..."))
		return Pick(text, options)
	}
	return options[index]
}
