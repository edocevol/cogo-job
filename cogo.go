package main

import (
	"gowork/job"
	"os"
)

//go:generate ./bin/init_cobra_job.sh

func main() {
	if err := job.CogoJob.Execute(); err != nil {
		os.Exit(-1)
	}
}
