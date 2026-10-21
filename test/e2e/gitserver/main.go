/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Command gitserver is the Git server of the kind harness of the fleet
// manager. It serves one bare repository over HTTPS through git
// http-backend, and takes pushes without credentials.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:])
	stop()

	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("gitserver", flag.ContinueOnError)
	root := flags.String("root", "/srv/git", "the directory that holds the repository")
	repository := flags.String("repository", "fleet.git", "the name of the bare repository")
	branch := flags.String("branch", "main", "the initial branch of the repository")
	listen := flags.String("listen", ":8443", "the address of the HTTPS server")
	tlsCert := flags.String("tls-cert", "", "the PEM certificate of the server (required)")
	tlsKey := flags.String("tls-key", "", "the PEM key of the server (required)")

	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parsing the flags: %w", err)
	}

	if *tlsCert == "" || *tlsKey == "" {
		return errors.New("flags --tls-cert and --tls-key are required")
	}

	if err := initRepository(ctx, *root, *repository, *branch); err != nil {
		return err
	}

	handler, err := newHandler(*root)
	if err != nil {
		return err
	}

	server := &http.Server{Addr: *listen, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.WithoutCancel(ctx))
	}()

	if err := server.ListenAndServeTLS(*tlsCert, *tlsKey); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving %s: %w", *listen, err)
	}

	return nil
}

// initRepository creates the bare repository root/repository with the
// initial branch, unless it exists, and lets HTTP clients push to it without
// a user.
func initRepository(ctx context.Context, root, repository, branch string) error {
	dir := filepath.Join(root, repository)

	if _, err := os.Stat(filepath.Join(dir, "HEAD")); errors.Is(err, os.ErrNotExist) {
		if out, err := exec.CommandContext(
			ctx, "git", "init", "--bare", "--initial-branch="+branch, dir,
		).CombinedOutput(); err != nil {
			return fmt.Errorf("initializing the repository %s: %w: %s", dir, err, out)
		}
	} else if err != nil {
		return fmt.Errorf("reading the repository %s: %w", dir, err)
	}

	// git http-backend refuses a push from an anonymous user unless the
	// repository sets http.receivepack.
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "config", "http.receivepack", "true").CombinedOutput()
	if err != nil {
		return fmt.Errorf("allowing pushes to the repository %s: %w: %s", dir, err, out)
	}

	return nil
}

// newHandler returns a handler that serves every repository under root with
// git http-backend.
func newHandler(root string) (http.Handler, error) {
	out, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		return nil, fmt.Errorf("finding the exec path of git: %w", err)
	}

	return &cgi.Handler{
		Path: filepath.Join(strings.TrimSpace(string(out)), "git-http-backend"),
		Env:  []string{"GIT_PROJECT_ROOT=" + root, "GIT_HTTP_EXPORT_ALL=1"},
	}, nil
}
