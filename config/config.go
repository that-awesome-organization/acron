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

type Config struct {
	TickerDuration string `json:"ticker_duration" yaml:"ticker_duration"`
	Jobs           []*Job `json:"jobs" yaml:"jobs"`
	LogFile        string `json:"log_file" yaml:"log_file"`

	logger *log.Logger
}

type Job struct {
	// Schedule string `json:"schedule" yaml:"schedule"`
	Rate    string   `json:"rate" yaml:"rate"`
	Command string   `json:"command" yaml:"command"`
	Args    []string `json:"args" yaml:"args"`
	Dir     string   `json:"dir" yaml:"dir"`
	EnvFile string   `json:"env_file" yaml:"env_file"`

	runLogs []*RunLog
}

type RunLog struct {
	runOn  time.Time
	result *exec.ExitError
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

	// if c.LogFile != "" {
	// 	o, err := os.OpenFile(c.LogFile, )
	// 	if
	// }

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
	if lastRun.Add(duration).Before(time.Now()) {
		go j.Run(ctx, logger)
		return true, nil
	}
	return false, nil
}

func (j *Job) Run(ctx context.Context, logger *log.Logger) {
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

	l := &RunLog{
		runOn: time.Now(),
	}

	err := cmd.Run()
	if err != nil {
		log.Println("Command failed with error:", err)
		if result, ok := err.(*exec.ExitError); ok {
			l.result = result
		}
	}
	logger.Printf("command: %s, time: %s, stdout: %q\n", j.Command, l.runOn.Format(time.RFC3339), string(stdoutBuf.Bytes()[:]))
	logger.Printf("command: %s, time: %s, stderr: %q\n", j.Command, l.runOn.Format(time.RFC3339), string(stderrBuf.Bytes()[:]))

	j.runLogs = append(j.runLogs, l)
}
