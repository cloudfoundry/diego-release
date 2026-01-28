package visualization

import (
	"fmt"
	"strings"
	"time"

	"github.com/onsi/gomega/format"
)

const defaultStyle = "\x1b[0m"
const redColor = "\x1b[91m"
const greenColor = "\x1b[32m"
const yellowColor = "\x1b[33m"
const cyanColor = "\x1b[36m"
const grayColor = "\x1b[90m"
const lightGrayColor = "\x1b[37m"
const purpleColor = "\x1b[35m"

func init() {
	format.UseStringerRepresentation = true
}

func cellID(index int) string {
	return fmt.Sprintf("REP-%d", index+1)
}

func PrintReport(report *Report) {
	if report.AuctionsPerformed() == 0 {
		fmt.Println("Got no results!")
		return
	}

	fmt.Printf("Finished %d Auctions (%d succeeded, %d failed) among %d Cells in %s\n", report.AuctionsPerformed(), len(report.AuctionResults.SuccessfulLRPs), len(report.AuctionResults.FailedLRPs), len(report.Cells), report.AuctionDuration)
	fmt.Println()

	auctionedInstances := map[string]bool{}
	for _, start := range report.AuctionResults.SuccessfulLRPs {
		auctionedInstances[start.Identifier()] = true
	}

	fmt.Println("Distribution")
	maxGuidLength := len(cellID(report.NReps() - 1))
	guidFormat := fmt.Sprintf("%%%ds", maxGuidLength)

	numNew := 0
	for i := 0; i < report.NReps(); i++ {
		cellIDString := fmt.Sprintf(guidFormat, cellID(i))

		instanceString := ""
		instances := report.InstancesByRep[cellID(i)]

		availableColors := []string{"red", "cyan", "yellow", "gray", "purple", "green"}
		colorLookup := map[string]string{"red": redColor, "green": greenColor, "cyan": cyanColor, "yellow": yellowColor, "gray": lightGrayColor, "purple": purpleColor}

		originalCounts := map[string]int{}
		newCounts := map[string]int{}
		totalUsage := 0
		for _, instance := range instances {
			key := "green"
			if _, ok := colorLookup[instance.ProcessGuid]; ok {
				key = instance.ProcessGuid
			}
			if auctionedInstances[instance.Identifier()] {
				newCounts[key] += int(instance.MemoryMB)
				numNew += 1
			} else {
				originalCounts[key] += int(instance.MemoryMB)
			}
			totalUsage += int(instance.MemoryMB)
		}
		for _, col := range availableColors {
			instanceString += strings.Repeat(colorLookup[col]+"-"+defaultStyle, originalCounts[col])
			instanceString += strings.Repeat(colorLookup[col]+"+"+defaultStyle, newCounts[col])
		}
		totalMemory := int(report.CellStates[cellID(i)].TotalResources.MemoryMB)
		instanceString += strings.Repeat(grayColor+"."+defaultStyle, totalMemory-totalUsage)

		fmt.Printf("  [%s] %s: %s\n", report.CellStates[cellID(i)].Zone, cellIDString, instanceString)
	}

	if numNew < report.NumAuctions {
		fmt.Printf("%s!!!!MISSING INSTANCES!!!!  Expected %d, got %d (%.3f %% failure rate)%s\n", redColor, report.NumAuctions, numNew, float64(report.NumAuctions-numNew)/float64(report.NumAuctions), defaultStyle)
	}

	for _, start := range report.AuctionResults.FailedLRPs {
		fmt.Printf("Failed: %s %d %d\n", start.Identifier(), start.MemoryMB, start.DiskMB)
	}

	waitDurations := []time.Duration{}
	for _, start := range report.AuctionResults.SuccessfulLRPs {
		waitDurations = append(waitDurations, start.WaitDuration)
	}
	minTime, maxTime, meanTime := StatsForDurations(waitDurations)
	fmt.Printf("%14s  Min: %16s | Max: %16s | Mean: %16s\n", "Wait Times:", minTime, maxTime, meanTime)

	minAttempts, maxAttempts, totalAttempts, meanAttempts := 100000000, 0, 0, float64(0)
	for _, start := range report.AuctionResults.SuccessfulLRPs {
		if start.Attempts < minAttempts {
			minAttempts = start.Attempts
		}
		if start.Attempts > maxAttempts {
			maxAttempts = start.Attempts
		}
		totalAttempts += start.Attempts
		meanAttempts += float64(start.Attempts)
	}

	for _, start := range report.AuctionResults.FailedLRPs {
		if start.Attempts < minAttempts {
			minAttempts = start.Attempts
		}
		if start.Attempts > maxAttempts {
			maxAttempts = start.Attempts
		}
		totalAttempts += start.Attempts
		meanAttempts += float64(start.Attempts)
	}

	meanAttempts = meanAttempts / float64(report.AuctionsPerformed())
	fmt.Printf("%14s  Min: %16d | Max: %16d | Mean: %16.2f | Total: %16d\n", "Attempts:", minAttempts, maxAttempts, meanAttempts, totalAttempts)
}

func StatsForDurations(durations []time.Duration) (time.Duration, time.Duration, time.Duration) {
	minTime, maxTime, meanTime := time.Hour, time.Duration(0), time.Duration(0)
	for _, duration := range durations {
		if duration < minTime {
			minTime = duration
		}
		if duration > maxTime {
			maxTime = duration
		}
		meanTime += duration
	}
	if len(durations) > 0 {
		meanTime = meanTime / time.Duration(len(durations))
	}

	return minTime, maxTime, meanTime
}
