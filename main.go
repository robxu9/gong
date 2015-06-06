package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

var (
	ErrNoRootDir               = errors.New("gong: no parent dir with .gong")
	ModeDirDefault os.FileMode = 0755
)

func execGo(gopath string, args ...string) int {
	cmd := exec.Command("go", args...)
	cmd.Env = os.Environ()
	for i, v := range cmd.Env {
		if strings.HasPrefix(v, "GOPATH=") {
			// remove the value
			cmd.Env = append(cmd.Env[:i], cmd.Env[i+1:]...)
		}
	}
	// and add our GOPATH
	if gopath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GOPATH=%s", gopath))
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus()
			}
			fmt.Printf("failed to end cleanly: %v\n", err)
			return 2
		} else {
			fmt.Printf("failed to end cleanly: %v\n", err)
			return 2
		}
	}

	return 0
}

// Attempt to traverse this directory and its parents until
// we come to a directory with a .gong file in it. If we can't
// find one by the time we come to /, we conclude it doesn't exist.
func rootDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	lastdir := ""

	for lastdir != dir {
		if _, err = os.Stat(".gong"); err == nil {
			return dir, nil
		}

		lastdir = dir
		dir = filepath.Dir(dir)
	}

	return "", ErrNoRootDir
}

// retrieve the gopath from the rootDir
func gopath() string {
	rootDir, _ := rootDir()
	return filepath.Join(rootDir, ".gong.deps")
}

// Attempt to setup the gong workspace, if we need to.
func setup(force bool) {
	rootDir, err := rootDir()
	if err == nil {
		if force {
			fmt.Printf("gong: running setup again for %s\n", rootDir)
		} else {
			// no need to do anything, just return
			return
		}
	} else if err != ErrNoRootDir {
		fmt.Printf("directory issues, can't continue: %v\n", err)
		os.Exit(2)
	} else {
		rootDir, err = os.Getwd()
		if err != nil {
			fmt.Printf("can't get working directory: %v\n", err)
			os.Exit(2)
		}
	}

	// create .gong.deps/{bin,pkg,src}
	fmt.Printf("setting up project in %s... ", rootDir)
	gongFile, err := os.OpenFile(filepath.Join(rootDir, ".gong"), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("failed! %v\n", err)
		os.Exit(2)
	}
	gongFile.Close()
	if err = os.MkdirAll(filepath.Join(rootDir, ".gong.deps", "src"), ModeDirDefault); err != nil {
		fmt.Printf("failed! %v\n", err)
		os.Exit(2)
	}
	if err = os.MkdirAll(filepath.Join(rootDir, ".gong.deps", "bin"), ModeDirDefault); err != nil {
		fmt.Printf("failed! %v\n", err)
		os.Exit(2)
	}
	if err = os.MkdirAll(filepath.Join(rootDir, ".gong.deps", "pkg"), ModeDirDefault); err != nil {
		fmt.Printf("failed! %v\n", err)
		os.Exit(2)
	}
	fmt.Printf("done.\n")

	// ask what the project name is going to be so we can create the symlink
	fmt.Printf("what is the project path going to be? (e.g. github.com/robxu9/gong)\n")
	path := ""

	scanner := bufio.NewScanner(os.Stdin)
	for path == "" {
		fmt.Printf("  > ")
		if scanner.Scan() {
			path = scanner.Text()
		}
		if err = scanner.Err(); err != nil {
			fmt.Printf("couldn't get the path... %v\n", err)
			os.Exit(2)
		}
		path = strings.TrimSpace(path)
		path = strings.TrimSuffix(path, "/")
		if strings.HasPrefix(path, "/") {
			fmt.Printf("-- can't start with a slash ('/')\n")
			path = ""
		}
		if strings.Contains(path, "//") {
			fmt.Printf("-- can't use double slashes ('//')\n")
			path = ""
		}
	}

	// okay, let's create that path.
	fmt.Printf("creating symlinks... ")
	// get the base directory first
	baseDir := filepath.Join(rootDir, ".gong.deps", "src", filepath.Dir(path))
	if err = os.MkdirAll(baseDir, ModeDirDefault); err != nil {
		fmt.Printf("failed! %v\n", err)
		os.Exit(2)
	}
	// now get the relative directory to make the symlink
	relDir, err := filepath.Rel(filepath.Join(rootDir, ".gong.deps", "src", filepath.Dir(path)), rootDir)
	if err != nil {
		fmt.Printf("failed! %v\n", err)
		os.Exit(2)
	}
	// and create the symlink
	if err = os.Symlink(relDir, filepath.Join(rootDir, ".gong.deps", "src", path)); err != nil {
		fmt.Printf("failed! %v\n", err)
		os.Exit(2)
	}
	fmt.Printf("done.\n")

	fmt.Printf("Setup should be completed now! Make sure to use gong as a wrapper\n")
	fmt.Printf("to your `go` commands so that you have environment variables set correctly.\n")

}

func help(failed bool) {
	fmt.Printf("gong is a nicer tool for managing Go source code\n\n")

	if str, err := rootDir(); err == nil {
		fmt.Printf("  -> in project %s\n\n", str)
	}

	fmt.Printf("Usage: gong command [args]\n\n")
	fmt.Printf("gong subcommands:\n")
	fmt.Printf("    setup                                  (re)Setup the project\n")
	fmt.Printf("Yep, that's it! If you want to manage dependencies, you can use\n")
	fmt.Printf("`gong get` just like `go get`, which will vendor it to your project.\n")
	fmt.Printf("Isn't that simple?\n")
	fmt.Printf("\n")
	fmt.Printf("and of course we support all of the go commands. `go help` follows:\n")
	execGo("", "help")

	if failed {
		os.Exit(2)
	}
	os.Exit(0)
}

func main() {
	if len(os.Args) <= 1 {
		help(true)
	}

	cmd := os.Args[1]
	retcode := 0

	switch cmd {
	case "setup":
		setup(true)
	case "help":
		if len(os.Args) <= 2 {
			help(false)
		}
		fallthrough
	default:
		setup(false)
		retcode = execGo(gopath(), os.Args[1:]...)
	}

	os.Exit(retcode)
}
