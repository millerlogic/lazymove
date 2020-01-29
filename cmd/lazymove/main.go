package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/millerlogic/lazymove"
)

func run() error {
	m := &lazymove.Mover{
		Timeout:    lazymove.DefaultTimeout,
		MinFileAge: lazymove.DefaultMinFileAge,
		MinDirAge:  lazymove.DefaultMinDirAge,
	}

	flag.DurationVar(&m.Timeout, "timeout", m.Timeout, "How often to look for files to move")
	flag.DurationVar(&m.MinFileAge, "minFileAge", m.MinFileAge, "Minimum age to move files")
	flag.DurationVar(&m.MinDirAge, "minDirAge", m.MinDirAge, "Minimum age to remove empty dirs")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [Options...] <SourceDir> <DestDir>\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.Arg(0) == "" || flag.Arg(1) == "" {
		flag.Usage()
		return errors.New("missing argument")
	}

	m.SourceDir = flag.Arg(0)
	m.DestDir = flag.Arg(1)

	return m.Run(context.Background())
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
