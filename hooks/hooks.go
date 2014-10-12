// This is the code that builds the hooks executable for the Discourse Juju
// charm.  To modify what code gets run (fix bugs, add debug output, etc),
// you'll need to install go, set up GOPATH, and get the dependencies.  The
// debugSetup.sh does all that for you.  Just source it by running
//
//   source ./debugsetup.sh
//
// And now your environment is set up to rebuild the hooks executable. To do so,
// make the modifications you want, and run
//
//   go build hooks.go
//
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v1"
)

const (
	dir = "/var/discourse"
)

var (
	appYml = filepath.Join(dir, "containers/app.yml")

	aptget  = runner("apt-get")
	aptkey  = runner("apt-key")
	git     = runner("git")
	service = runner("service")

	launcher = bash(filepath.Join(dir, "launcher"))
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println(os.Stderr, "usage: hooks [install || config-changed]")
		os.Exit(1)
	}
	if err := Main(os.Args[1]); err != nil {
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

	case "upgrade-charm":
		fmt.Printf("Ignoring hook: %q\n", hook)
		return nil

	case "start", "stop":
		return fmt.Errorf("Hook %q should not execute via hooks.go.", hook)

	default:
		return fmt.Errorf("Unknown hook: %q\n", hook)
	}
}

func install() error {
	fmt.Println("Installing Discourse.")

	if err := installDocker(); err != nil {
		return err
	}

	fmt.Println("Installing git...")
	if err := aptget("install", "-y", "git"); err != nil {
		return fmt.Errorf("failed installing git: %s", err)
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
		fmt.Println("Discourse exists, no need to git clone.")
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

	dc, err := parseDiscourseConfig(data)

	data, err = yaml.Marshal(dc)
	if err != nil {
		return fmt.Errorf("Error exporting config from yaml: %s", err)
	}

	if err := ioutil.WriteFile(appYml, data, 0644); err != nil {
		return fmt.Errorf("Error writing app.yml: %s", err)
	}

	// Now apply any configuration settings specified at deploy time.
	cfg, err := getCharmConfig()
	if err != nil {
		return err
	}

	if _, err := writeNewConfig(cfg); err != nil {
		return err
	}

	if err := open(80); err != nil {
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

	fmt.Println("Finished installing discourse.")

	return nil
}

type charmConfig struct {
	DISCOURSE_DEVELOPER_EMAILS *string `json:"DISCOURSE_DEVELOPER_EMAILS,omitempty"`
	DISCOURSE_SMTP_ADDRESS     *string `json:"DISCOURSE_SMTP_ADDRESS,omitempty"`
	DISCOURSE_HOSTNAME         *string `json:"DISCOURSE_HOSTNAME,omitempty"`
	DISCOURSE_SMTP_PORT        *int    `json:"DISCOURSE_SMTP_PORT,omitempty"`
	DISCOURSE_SMTP_USER_NAME   *string `json:"DISCOURSE_SMTP_USER_NAME,omitempty"`
	DISCOURSE_SMTP_PASSWORD    *string `json:"DISCOURSE_SMTP_PASSWORD,omitempty"`
	UNICORN_WORKERS            *int    `json:"UNICORN_WORKERS,omitempty"`
	DISCOURSE_CDN_URL          *string `json:"DISCOURSE_CDN_URL,omitempty"`
}

type discourseConfig map[interface{}]interface{}

func configChanged() error {
	cfg, err := getCharmConfig()
	if err != nil {
		return err
	}

	changed, err := writeNewConfig(cfg)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Println("No config changes detected.")
		return nil
	}
	fmt.Println("Config changes dectected. Rebuilding discourse container...")
	if err := launcher("rebuild", "app"); err != nil {
		return fmt.Errorf("Error rebuilding discourse: %s", err)
	}
	return nil
}

func getCharmConfig() ([]byte, error) {
	out, err := exec.Command("config-get", "--format", "json").CombinedOutput()
	if err != nil {
		fmt.Fprintln(os.Stderr, out)
		return nil, fmt.Errorf("Error calling config-get: %s", err)
	}
	return out, nil
}

func writeNewConfig(b []byte) (changed bool, err error) {
	if len(b) == 0 {
		fmt.Println("No config set.")
		return false, nil
	}

	fmt.Println("Updating config.")

	cc, err := parseCharmConfig(b)
	if err != nil {
		return false, err
	}

	fileContents, err := ioutil.ReadFile(appYml)
	if err != nil {
		return false, fmt.Errorf("Can't read discourse config file: %s", err)
	}

	dc, err := parseDiscourseConfig(fileContents)
	if err != nil {
		return false, err
	}

	dc, err = merge(dc, cc)
	if err != nil {
		return false, err
	}

	newContents, err := yaml.Marshal(dc)
	if err != nil {
		return false, fmt.Errorf("Can't marshal app.yaml changes: %s", err)
	}

	if bytes.Equal(fileContents, newContents) {
		return false, nil
	}
	if err := ioutil.WriteFile(appYml, newContents, 0644); err != nil {
		return true, fmt.Errorf("Error writing app.yaml changes: %s", err)
	}

	return true, nil
}

func parseCharmConfig(b []byte) (charmConfig, error) {
	cfg := charmConfig{}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("Can't parse charm config: %s", err)
	}
	return cfg, nil
}

func parseDiscourseConfig(b []byte) (discourseConfig, error) {
	cfg := discourseConfig{}
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return cfg, fmt.Errorf("Error unmarshalling app.yml: %s", err)
	}
	return cfg, nil
}

func merge(dc discourseConfig, cc charmConfig) (discourseConfig, error) {
	// env is a sub-map in the yaml where our values get stored.
	env := map[interface{}]interface{}{}
	if dc["env"] == nil {
		dc["env"] = env
	} else {
		var ok bool
		env, ok = dc["env"].(map[interface{}]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected type for env key in discourse config: %T", dc["env"])
		}
	}

	if cc.DISCOURSE_DEVELOPER_EMAILS != nil {
		env["DISCOURSE_DEVELOPER_EMAILS"] = *cc.DISCOURSE_DEVELOPER_EMAILS
	}
	if cc.DISCOURSE_SMTP_ADDRESS != nil {
		env["DISCOURSE_SMTP_ADDRESS"] = *cc.DISCOURSE_SMTP_ADDRESS
	}
	if cc.DISCOURSE_HOSTNAME != nil {
		env["DISCOURSE_HOSTNAME"] = *cc.DISCOURSE_HOSTNAME
	}
	if cc.DISCOURSE_SMTP_PORT != nil {
		env["DISCOURSE_SMTP_PORT"] = *cc.DISCOURSE_SMTP_PORT
	}
	if cc.DISCOURSE_SMTP_USER_NAME != nil {
		env["DISCOURSE_SMTP_USER_NAME"] = *cc.DISCOURSE_SMTP_USER_NAME
	}
	if cc.DISCOURSE_SMTP_PASSWORD != nil {
		env["DISCOURSE_SMTP_PASSWORD"] = *cc.DISCOURSE_SMTP_PASSWORD
	}
	if cc.UNICORN_WORKERS != nil {
		env["UNICORN_WORKERS"] = *cc.UNICORN_WORKERS
	}
	if cc.DISCOURSE_CDN_URL != nil {
		env["DISCOURSE_CDN_URL"] = *cc.DISCOURSE_CDN_URL
	}
	return dc, nil
}

func installDocker() error {
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
	err = aptget("update")
	if err != nil {
		return fmt.Errorf("failed running apt-get update: %s", err)
	}

	if err := installAufs(); err != nil {
		return err
	}

	fmt.Println("Installing apt-transport-https...")
	if err := aptget("install", "-y", "apt-transport-https"); err != nil {
		return fmt.Errorf("failed installing apt-transport-https: %s", err)
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

	return nil
}

func installAufs() error {
	uname, err := exec.Command("uname", "-r").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed running uname -r: %s", err)
	}

	// The output from uname may contain a line return, so we need to strip that
	// out.
	extra := "linux-image-extra-" + strings.TrimSpace(string(uname))
	fmt.Printf("Installing %s...\n", extra)
	if err := aptget("install", "-y", extra); err != nil {
		return fmt.Errorf("failed installing %s: %s", extra, err)
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

func bash(args ...string) func(args ...string) error {
	return func(moreArgs ...string) error {
		cmd := exec.Command("bash", append(args, moreArgs...)...)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout

		// Open a stdinpipe and then close it, which tells the script there will
		// be no user input.
		in, err := cmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("error creating stdinpipe: %s", err)
		}
		if err := cmd.Start(); err != nil {
			return err
		}
		in.Close()
		return cmd.Wait()
	}
}

func open(port int) error {
	out, err := exec.Command("open-port", strconv.Itoa(port)).CombinedOutput()
	if err != nil {
		// Ignore the error that indicates the port is already open.
		if !strings.Contains(string(out), "due to conflict") {
			return fmt.Errorf("failed running open-port %d: %s", port, err)
		}
	}
	return nil
}
