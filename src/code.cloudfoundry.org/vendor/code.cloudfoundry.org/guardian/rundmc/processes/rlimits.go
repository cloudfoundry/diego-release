package processes

import (
	"code.cloudfoundry.org/garden"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func toRlimits(rlimits garden.ResourceLimits) (results []specs.POSIXRlimit) {
	if rlimits.As != nil {
		results = append(results, specs.POSIXRlimit{
			Type: "RLIMIT_AS",
			Soft: *rlimits.As,
			Hard: *rlimits.As,
		})
	}
	if rlimits.Core != nil {
		results = append(results, specs.POSIXRlimit{
			Type: "RLIMIT_CORE",
			Soft: *rlimits.Core,
			Hard: *rlimits.Core,
		})
	}
	if rlimits.Cpu != nil {
		results = append(results, specs.POSIXRlimit{
			Type: "RLIMIT_CPU",
			Soft: *rlimits.Cpu,
			Hard: *rlimits.Cpu,
		})
	}
	if rlimits.Data != nil {
		results = append(results, specs.POSIXRlimit{
			Type: "RLIMIT_DATA",
			Soft: *rlimits.Data,
			Hard: *rlimits.Data,
		})
	}
	if rlimits.Fsize != nil {
		results = append(results, specs.POSIXRlimit{
			Type: "RLIMIT_FSIZE",
			Soft: *rlimits.Fsize,
			Hard: *rlimits.Fsize,
		})
	}
	if rlimits.Locks != nil {
		results = append(results, specs.POSIXRlimit{
			Type: "RLIMIT_LOCKS",
			Soft: *rlimits.Locks,
			Hard: *rlimits.Locks,
		})
	}
	if rlimits.Memlock != nil {
		results = append(results, specs.POSIXRlimit{
			Type: "RLIMIT_MEMLOCK",
			Soft: *rlimits.Memlock,
			Hard: *rlimits.Memlock,
		})
	}
	if rlimits.Msgqueue != nil {
		results = append(results, specs.POSIXRlimit{
			Type: "RLIMIT_MSGQUEUE",
			Soft: *rlimits.Msgqueue,
			Hard: *rlimits.Msgqueue,
		})
	}
	if rlimits.Nice != nil {
		results = append(results, specs.POSIXRlimit{
			Type: "RLIMIT_NICE",
			Soft: *rlimits.Nice,
			Hard: *rlimits.Nice,
		})
	}
	if rlimits.Nofile != nil {
		results = append(results, specs.POSIXRlimit{
			Type: "RLIMIT_NOFILE",
			Soft: *rlimits.Nofile,
			Hard: *rlimits.Nofile,
		})
	}
	if rlimits.Nproc != nil {
		results = append(results, specs.POSIXRlimit{
			Type: "RLIMIT_NPROC",
			Soft: *rlimits.Nproc,
			Hard: *rlimits.Nproc,
		})
	}
	if rlimits.Rss != nil {
		results = append(results, specs.POSIXRlimit{
			Type: "RLIMIT_RSS",
			Soft: *rlimits.Rss,
			Hard: *rlimits.Rss,
		})
	}
	if rlimits.Rtprio != nil {
		results = append(results, specs.POSIXRlimit{
			Type: "RLIMIT_RTPRIO",
			Soft: *rlimits.Rtprio,
			Hard: *rlimits.Rtprio,
		})
	}
	if rlimits.Sigpending != nil {
		results = append(results, specs.POSIXRlimit{
			Type: "RLIMIT_SIGPENDING",
			Soft: *rlimits.Sigpending,
			Hard: *rlimits.Sigpending,
		})
	}
	if rlimits.Stack != nil {
		results = append(results, specs.POSIXRlimit{
			Type: "RLIMIT_STACK",
			Soft: *rlimits.Stack,
			Hard: *rlimits.Stack,
		})
	}

	return results
}
