package main

import (
	"log"
	"fmt"
	"strings"
)


type CronLogger struct {
    *log.Logger
}

func (CronLogger CronLogger) Verbose(message string) {
    if opts.Verbose {
        CronLogger.Println(message)
    }
}

func (CronLogger CronLogger) cronjobToString(cronjob CrontabEntry) string {
    parts := []string{}

    parts = append(parts, fmt.Sprintf("spec:'%v'", cronjob.Spec))
    parts = append(parts, fmt.Sprintf("usr:%v", cronjob.User))
    parts = append(parts, fmt.Sprintf("cmd:'%v'", cronjob.Command))

    if len(cronjob.Env) >= 1 {
        parts = append(parts, fmt.Sprintf("env:'%v'", cronjob.Env))
    }

    return strings.Join(parts," ")
}

func (CronLogger CronLogger) CronjobAdd(cronjob CrontabEntry) {
    CronLogger.Printf("add: %v\n", CronLogger.cronjobToString(cronjob))
}

func (CronLogger CronLogger) CronjobExec(cronjob CrontabEntry) {
    if opts.Verbose {
        CronLogger.Printf("exec: %v\n", CronLogger.cronjobToString(cronjob))
    }
}

func (CronLogger CronLogger) CronjobExecFailed(cronjob CrontabEntry, output string, err error) {
    CronLogger.Printf("failed cronjob: cmd:%v out:%v err:%v\n", cronjob.Command, output, err)
}

func (CronLogger CronLogger) CronjobExecSuccess(cronjob CrontabEntry) {
    if opts.Verbose {
        CronLogger.Printf("ok: %v\n", CronLogger.cronjobToString(cronjob))
    }
}
