package lagerflags

import "strings"

type RedactPatterns []string

func (p *RedactPatterns) String() string {
	return strings.Join(*p, ",")
}

func (p *RedactPatterns) Set(value string) error {
	*p = append(*p, value)
	return nil
}
