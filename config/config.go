package config

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"time"
)

var (
	FirstKey = ctxKey("first")
)

type ctxKey string

type Config struct {
	Address        string `json:"address" yaml:"address"`
	TickerDuration string `json:"ticker_duration" yaml:"ticker_duration"`
	Jobs           []*Job `json:"jobs" yaml:"jobs"`
	LogFile        string `json:"log_file" yaml:"log_file"`

	logger *log.Logger
}

type Job struct {
	// Schedule string `json:"schedule" yaml:"schedule"`
	Name     string   `json:"name" yaml:"name"`
	Rate     string   `json:"rate" yaml:"rate"`
	Delay    string   `json:"delay" yaml:"delay"`
	Command  string   `json:"command" yaml:"command"`
	Args     []string `json:"args" yaml:"args"`
	Dir      string   `json:"dir" yaml:"dir"`
	EnvFile  string   `json:"env_file" yaml:"env_file"`
	Disabled bool     `json:"disabled" yaml:"disabled"`

	runLogs    []*RunLog
	currentLog *RunLog
}

type RunLog struct {
	runOn     time.Time
	timeTaken time.Duration
	result    *exec.ExitError
	stdoutLog string
	stderrLog string
}

func (j *Job) GetLastRunOn() *time.Time {
	if len(j.runLogs) > 0 {
		return &j.runLogs[len(j.runLogs)-1].runOn
	}
	return nil
}

func (j *Job) GetLastRunLog(outputType string) string {
	var runLog *RunLog
	if len(j.runLogs) > 0 {
		runLog = j.runLogs[len(j.runLogs)-1]
	}
	switch outputType {
	case "stdout":
		return runLog.stdoutLog
	case "stderr":
		return runLog.stderrLog
	default:
		return "invalid output type"
	}
}

func (j *Job) GetLastRunDuration() time.Duration {
	var runLog *RunLog
	if len(j.runLogs) > 0 {
		runLog = j.runLogs[len(j.runLogs)-1]
	}
	return runLog.timeTaken
}

func (c *Config) Init() error {
	var output io.Writer
	if c.LogFile != "" {
		o, err := initLogFile(c.LogFile)
		if err == nil {
			output = o
		} else {
			log.Println("error initializing logger:", err)
		}
	}
	if output == nil {
		output = os.Stdout
	}

	c.logger = log.New(output, "[acron] ", log.LstdFlags|log.Lshortfile)

	return nil
}

func initLogFile(logfile string) (io.Writer, error) {
	_, err := os.Stat(logfile)
	if errors.Is(err, fs.ErrNotExist) {
		return os.Create(logfile)
	}

	if err := os.Rename(logfile, fmt.Sprintf("%s.%d", logfile, time.Now().UnixMilli())); err != nil {
		log.Println("error renaming file")
		return nil, err
	}
	return os.Create(logfile)
}

func (c *Config) Check(ctx context.Context) error {
	for _, job := range c.Jobs {
		b, err := job.Check(ctx, c.logger)
		if err != nil {
			log.Println(err)
			return err
			// continue
		}
		if b {
			log.Printf("Triggered %s", job.Command)
		}
	}
	return nil
}

func (j *Job) Check(ctx context.Context, logger *log.Logger) (bool, error) {
	if j.Disabled {
		log.Printf("%s job is disabled, skipping", j.Name)
		return false, nil
	}
	var lastRun time.Time
	if len(j.runLogs) > 0 {
		lastRun = j.runLogs[len(j.runLogs)-1].runOn
	}

	var duration time.Duration
	if j.Rate != "" {
		d, err := time.ParseDuration(j.Rate)
		if err != nil {
			log.Println(err)
			return false, err
		}
		duration = d
	}

	// parse the delay value for the first time to delay the execution
	if val, ok := ctx.Value(FirstKey).(bool); val && ok {
		if j.Delay != "" {
			d, err := time.ParseDuration(j.Delay)
			if err == nil {
				lastRun = time.Now().Add(d)
			} else {
				log.Println("error parsing delay:", j.Delay, err)
			}
		}
	}

	if lastRun.Add(duration).Before(time.Now()) && // check if the command should be run
		j.currentLog == nil { // checks if the command is not currently running
		go j.Run(ctx, logger)
		return true, nil
	}
	return false, nil
}

func (j *Job) Run(ctx context.Context, logger *log.Logger) {
	j.currentLog = &RunLog{
		runOn: time.Now(),
	}
	cmd := exec.CommandContext(ctx, j.Command, j.Args...)
	cmd.Env = append(cmd.Env, "ACRON_EXEC=1")
	if j.EnvFile != "" {
		f, err := os.Open(j.EnvFile)
		if err != nil {
			log.Println("error reading environment file:", err)
		}
		s := bufio.NewScanner(f)
		for s.Scan() {
			if s.Err() == io.EOF {
				break
			}
			cmd.Env = append(cmd.Env, s.Text())
		}
	}
	stdoutBuf := bytes.Buffer{}
	stderrBuf := bytes.Buffer{}
	cmd.Dir = j.Dir
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err != nil {
		log.Println("Command failed with error:", err)
		if result, ok := err.(*exec.ExitError); ok {
			j.currentLog.result = result
		}
	}

	logger.Printf("command: %s, time: %s, stdout: %q\n", j.Command, j.currentLog.runOn.Format(time.RFC3339), string(stdoutBuf.Bytes()[:]))
	logger.Printf("command: %s, time: %s, stderr: %q\n", j.Command, j.currentLog.runOn.Format(time.RFC3339), string(stderrBuf.Bytes()[:]))

	j.currentLog.stdoutLog = string(stdoutBuf.Bytes()[:])
	j.currentLog.stderrLog = string(stderrBuf.Bytes()[:])
	j.currentLog.timeTaken = time.Since(j.currentLog.runOn)
	j.runLogs = append(j.runLogs, j.currentLog)
	j.currentLog = nil
}
