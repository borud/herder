package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	"log/syslog"

	"github.com/shirou/gopsutil/process"
)

var name = flag.String("name", "", "Process name to monitor")
var loadFactor = flag.Float64("limit", 0.7, "Load limit")
var grace = flag.Int("grace", 20, "Grace period after kill in seconds")
var samples = flag.Int("samples", 3, "Number of times in a row process needs to be over the limit before killed")

func init() {
	logwriter, err := syslog.New(syslog.LOG_ERR, "herder")
	if err != nil {
		log.Printf("Got error setting up syslog: %v", err)
		return
	}
	// The syslog entries already contains a timestamp. Just include the
	// location of the log message.
	log.SetFlags(log.Lshortfile)
	log.SetOutput(logwriter)
}

func getSeconds(p *process.Process) (s float64, err error) {
	t, err := p.Times()
	if err != nil {
		return 0.0, errors.New("Couldn't get times")
	}

	return t.Total(), nil
}

func getProcess(name *string) (*process.Process, error) {
	pids, err := process.Pids()
	if err != nil {
		return nil, err
	}

	for _, pid := range pids {
		p, err := process.NewProcess(pid)
		if err != nil {
			continue
		}

		var n, _ = p.Name()
		if n == *name {
			return p, nil
		}
	}
	return nil, errors.New("Couldn't find process")
}

func calcDelta(p *process.Process, t time.Duration) (float64, error) {
	start, err := getSeconds(p)
	if err != nil {
		return 0.0, err
	}

	time.Sleep(t)

	end, err := getSeconds(p)
	if err != nil {
		return 0.0, err
	}

	return end - start, nil
}

func main() {
	flag.Parse()

	if *name == "" {
		fmt.Println("\nPlease specify at least -name parameter:")
		flag.PrintDefaults()
		fmt.Println()
		return
	}

	var overloadCountdown = *samples

	for {
		time.Sleep(time.Duration(2) * time.Second)

		p, err := getProcess(name)
		if err != nil {
			continue
		}

		for {
			delta, err := calcDelta(p, time.Duration(1)*time.Second)
			if err != nil {
				break
			}

			if delta >= *loadFactor {
				overloadCountdown--
			} else {
				// Reset countdown if we dip below load factor
				overloadCountdown = *samples
			}

			if overloadCountdown == 0 {
				p.Kill()
				overloadCountdown = *samples
				log.Printf("Killed %s because load factor was more than %f", *name, *loadFactor)
				time.Sleep(time.Duration(*grace) * time.Second)
			}

			time.Sleep(time.Duration(1) * time.Second)
		}
	}
}
