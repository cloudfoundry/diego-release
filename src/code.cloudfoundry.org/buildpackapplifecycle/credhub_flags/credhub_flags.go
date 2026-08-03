package credhub_flags

import (
	"flag"
	"time"
)

const (
	credhubConnectAttemptsFlag    = "credhubConnectAttempts"
	credhubRetryDelayFlag         = "credhubRetryDelay"
	credhubConnectAttemptsDefault = 3
	credhubRetryDelayDefault      = 1 * time.Second
)

type CredhubFlags struct {
	*flag.FlagSet
}

func NewCredhubFlags(component string) CredhubFlags {
	flagSet := flag.NewFlagSet(component, flag.ExitOnError)

	AddCredhubFlags(flagSet)

	return CredhubFlags{
		FlagSet: flagSet,
	}
}

func (chf CredhubFlags) ConnectAttempts() int {
	return ConnectAttempts(chf.FlagSet)
}

func (chf CredhubFlags) RetryDelay() time.Duration {
	return RetryDelay(chf.FlagSet)
}

func AddCredhubFlags(flagSet *flag.FlagSet) {
	flagSet.Int(
		credhubConnectAttemptsFlag,
		credhubConnectAttemptsDefault,
		"number of times that the credhub client will attempt to connect to credhub",
	)

	flagSet.Duration(
		credhubRetryDelayFlag,
		credhubRetryDelayDefault,
		"delay duration that the credhub client will wait before retrying the connection to credhub",
	)
}

func ConnectAttempts(flagSet *flag.FlagSet) int {
	return flagSet.Lookup(credhubConnectAttemptsFlag).Value.(flag.Getter).Get().(int)
}

func RetryDelay(flagSet *flag.FlagSet) time.Duration {
	return flagSet.Lookup(credhubRetryDelayFlag).Value.(flag.Getter).Get().(time.Duration)
}
