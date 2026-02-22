package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexflint/go-arg"
)

type Subcommand interface {
	Do(context.Context, *arg.Parser, *http.Client, Args) error
}

type Args struct {
	Port int `arg:"-p,--port" default:"1274"`
}

type Commands struct {
	Args
	User *UserCmd `arg:"subcommand:user"`
}

func main() {
	var args Commands
	p := arg.MustParse(&args)
	sub, ok := p.Subcommand().(Subcommand)
	if !ok {
		p.Fail("missing subcommand")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cli := &http.Client{
		Timeout: 10 * time.Second,
	}
	err := sub.Do(ctx, p, cli, args.Args)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
