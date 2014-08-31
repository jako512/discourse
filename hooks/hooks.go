package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

var (
	aptget   = runner("apt-get")
	git      = runner("git")
	launcher = runner("launcher")
)

func main() {
	if err := Main(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func Main() error {
	switch os.Args(0) {
	case INSTALL:
		return install()
	}
	return fmt.Errorf("Unknown hook: %s", os.Args(0))
}

func install() error {
	if err := aptget("install", "git"); err != nil {
		return fmt.Errorf("failed installing git: %s, output: %s", err, out)
	}

	if err := aptget("install", "docker.io"); err != nil {
		return fmt.Errorf("failed installing docker: %s, output: %s", err, out)
	}

	if err := os.Symlink("/usr/bin/docker.io", "/usr/local/bin/docker"); err != nil {
		return fmt.Errorf("failed creating symlink for docker: %s", err)
	}

	dir := "/var/discourse"
	if err := os.Mkdir(dir, 0744); err != nil {
		return fmt.Errorf("failed creating directory for docker: %s", err)
	}

	err := git("clone", "https://github.com/discourse/discourse_docker.git", dir)
	if err != nil {
		return fmt.Errorf("failed git clone of discourse_docker repo: %s, output: %s", err, out)
	}

	dst := filepath.Join(dir, "containers/app.yml")
	src := filepath.Join(dir, "samples/standalone.yml")
	if err := copy(dst, src); err != nil {
		return fmt.Errorf("Error copying standalone.yml to containers/app.yml: %s", err)
	}

	if err := launcher("bootstrap", "app"); err != nil {
		return fmt.Errorf("Error running discourse bootstrap: %s", err)
	}

	return nil
}

func runner(name string) func(args ...string) error {
	return func(args ...string) error {
		cmd := exec.Command(name, args...)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		return cmd.Run()
	}
}

func copy(dst, src string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	newf, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer newf.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return nil
}
