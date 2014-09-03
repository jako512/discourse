// This is the code that builds the hooks executable for the Discourse Juju
// charm.  To modify what code gets run, and/or debug on the deployed unit,
// you'll need to install go, set up GOPATH, and get the dependencies:
//
//   apt-get install golang-go git
//   export GOPATH=$HOME
//   go get gopkg.in/yaml.v1
//
// Now you can build the hooks file:
//
//   go build hooks.go
//
// Or just run it directly like a script:
//
//   go run hooks.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v1"
)

const (
	dir = "/var/discourse"
)

var (
	appYml = filepath.Join(dir, "containers/app.yml")

	aptget   = runner("apt-get")
	aptkey   = runner("apt-key")
	git      = runner("git")
	service  = runner("service")
	launcher = runner(filepath.Join(dir, "launcher"))
)

func main() {
	if err := Main(filepath.Base(os.Args[0])); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func Main(hook string) error {
	switch hook {
	case "install":
		return install()

	case "config-changed":
		return configChanged()

	case "start", "upgrade-charm", "stop":
		fmt.Printf("Ignoring hook: %q\n", hook)
		return nil

	default:
		return fmt.Errorf("Unknown hook: %q\n", hook)
	}
}

func install() error {
	fmt.Println("Installing git...")
	if err := aptget("install", "-y", "git"); err != nil {
		return fmt.Errorf("failed installing git: %s", err)
	}

	fmt.Println("Installing apt-transport-https...")
	if err := aptget("install", "-y", "apt-transport-https"); err != nil {
		return fmt.Errorf("failed installing apt-transport-https: %s", err)
	}

	fmt.Println("Adding docker key...")
	err := aptkey("adv", "--keyserver", "hkp://keyserver.ubuntu.com:80", "--recv-keys", "36A1D7869245C8950F966E92D8576A8BA88D21E9")
	if err != nil {
		return fmt.Errorf("failed adding docker key: %s", err)
	}

	fmt.Println("Writing docker deb list...")
	err = ioutil.WriteFile("/etc/apt/sources.list.d/docker.list", []byte("deb https://get.docker.io/ubuntu docker main"), 0644)
	if err != nil {
		return fmt.Errorf("failed writing docker.list: %s", err)
	}

	fmt.Println("Calling apt-get update...")
	if err := aptget("update"); err != nil {
		return fmt.Errorf("failed running apt-get update: %s", err)
	}

	fmt.Println("Installing docker...")
	if err := aptget("install", "-y", "lxc-docker"); err != nil {
		return fmt.Errorf("failed installing lxc-docker: %s", err)
	}

	fmt.Println("Symlinking docker...")
	if _, err := os.Readlink("/usr/local/bin/docker"); err != nil {
		if err := os.Symlink("/usr/bin/docker.io", "/usr/local/bin/docker"); err != nil {
			return fmt.Errorf("failed creating symlink for docker: %s", err)
		}
	}

	fmt.Println("Creating discourse directory...")
	if err := os.MkdirAll(dir, 0744); err != nil {
		return fmt.Errorf("failed creating directory for discourse: %s", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		fmt.Println("Git cloning discourse...")
		err := git("clone", "https://github.com/discourse/discourse_docker.git", dir)
		if err != nil {
			return fmt.Errorf("failed git clone of discourse_docker repo: %s", err)
		}
	} else {
		fmt.Println("Discourse already exists.")
	}

	fmt.Println("Copying standalone.yml to app.yml...")
	data, err := ioutil.ReadFile(filepath.Join(dir, "samples/standalone.yml"))
	if err != nil {
		return fmt.Errorf("Can't read initial yaml: %s", err)
	}

	// This lets us fool the bootstrap script into bootstrapping without real
	// email info if it isn't supplied at bootstrap time. We can replace it
	// later via config-set.
	data = bytes.Replace(data, []byte("smtp.example.com"), []byte("foo.example.com"), -1)

	// run it through yaml so we have the same output format as we will when the
	// config changes.
	vals := map[string]interface{}{}
	yaml.Unmarshal(data, &vals)

	data, err = yaml.Marshal(vals)
	if err != nil {
		return fmt.Errorf("Error exporting config from yaml: %s", err)
	}

	if err := ioutil.WriteFile(appYml, data, 0644); err != nil {
		return fmt.Errorf("Error writing app.yml: %s", err)
	}

	// Now apply any configuration settings specified at deploy time.
	if _, err := writeNewConfig(); err != nil {
		return err
	}

	fmt.Println("Bootstrapping discourse...")
	if err := launcher("bootstrap", "app"); err != nil {
		return fmt.Errorf("Error running discourse bootstrap: %s", err)
	}

	fmt.Println("Starting discourse...")
	if err := launcher("start", "app"); err != nil {
		return fmt.Errorf("Error starting discourse: %s", err)
	}
	return nil
}

type config struct {
	DISCOURSE_DEVELOPER_EMAILS *string `json:"DISCOURSE_DEVELOPER_EMAILS,omitempty"`
	DISCOURSE_SMTP_ADDRESS     *string `json:"DISCOURSE_SMTP_ADDRESS,omitempty"`
	DISCOURSE_SMTP_PORT        *int    `json:"DISCOURSE_SMTP_PORT,omitempty"`
	DISCOURSE_SMTP_USER_NAME   *string `json:"DISCOURSE_SMTP_USER_NAME,omitempty"`
	DISCOURSE_SMTP_PASSWORD    *string `json:"DISCOURSE_SMTP_PASSWORD,omitempty"`
	UNICORN_WORKERS            *int    `json:"UNICORN_WORKERS,omitempty"`
	DISCOURSE_CDN_URL          *string `json:"DISCOURSE_CDN_URL,omitempty"`
}

func configChanged() error {
	changed, err := writeNewConfig()
	if err != nil {
		return err
	}
	if !changed {
		fmt.Println("No config changes detected.")
		return nil
	}
	fmt.Println("Config changes dectected. Restarting discourse...")
	if err := launcher("restart", "app"); err != nil {
		return fmt.Errorf("Error restarting discourse: %s", err)
	}
	return nil
}

func writeNewConfig() (changed bool, err error) {
	out, err := exec.Command("config-get", "--format", "json").CombinedOutput()
	if err != nil {
		fmt.Fprintln(os.Stderr, out)
		return false, fmt.Errorf("Error calling config-get: %s", err)
	}

	cfg := config{}
	if err := json.Unmarshal(out, &cfg); err != nil {
		return false, fmt.Errorf("Can't parse output from config-get: %s", err)
	}

	fileContents, err := ioutil.ReadFile(appYml)
	if err != nil {
		return false, fmt.Errorf("Can't read discourse config file: %s", err)
	}

	vals := map[string]interface{}{}
	yaml.Unmarshal(fileContents, &vals)

	// env is a sub-map in the yaml where our values get stored.
	env := map[string]interface{}{}
	if v, ok := vals["env"]; ok {
		if v, ok := v.(map[string]interface{}); ok {
			env = v
		}
	}

	if cfg.DISCOURSE_DEVELOPER_EMAILS != nil {
		emails := strings.Split(*cfg.DISCOURSE_DEVELOPER_EMAILS, ",")
		env["DISCOURSE_DEVELOPER_EMAILS"] = emails
	}
	if cfg.DISCOURSE_SMTP_ADDRESS != nil {
		env["DISCOURSE_SMTP_ADDRESS"] = *cfg.DISCOURSE_SMTP_ADDRESS
	}
	if cfg.DISCOURSE_SMTP_PORT != nil {
		env["DISCOURSE_SMTP_PORT"] = *cfg.DISCOURSE_SMTP_PORT
	}
	if cfg.DISCOURSE_SMTP_USER_NAME != nil {
		env["DISCOURSE_SMTP_USER_NAME"] = *cfg.DISCOURSE_SMTP_USER_NAME
	}
	if cfg.DISCOURSE_SMTP_PASSWORD != nil {
		env["DISCOURSE_SMTP_PASSWORD"] = *cfg.DISCOURSE_SMTP_PASSWORD
	}
	if cfg.UNICORN_WORKERS != nil {
		env["UNICORN_WORKERS"] = *cfg.UNICORN_WORKERS
	}
	if cfg.DISCOURSE_CDN_URL != nil {
		env["DISCOURSE_CDN_URL"] = *cfg.DISCOURSE_CDN_URL
	}

	fmt.Printf("Updating config: %v\n", env)

	vals["env"] = env

	newContents, err := yaml.Marshal(vals)
	if err != nil {
		return false, fmt.Errorf("Can't marshal app.yaml changes: %s", err)
	}

	if bytes.Equal(fileContents, newContents) {
		return false, nil
	}
	if err := ioutil.WriteFile(appYml, out, 0644); err != nil {
		return true, fmt.Errorf("Error writing app.yaml changes: %s", err)
	}
	return true, nil
}

func runner(name string) func(args ...string) error {
	return func(args ...string) error {
		cmd := exec.Command(name, args...)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		return cmd.Run()
	}
}
