package main

import (
	"fmt"
	"os"
	"strings"

	util "github.com/kopoli/go-util"
	"gopkg.in/src-d/go-git.v4"
)

var (
	majorVersion     = "0"
	version          = "Undefined"
	timestamp        = "Undefined"
	progVersion      = majorVersion + "-" + version
	exitValue    int = 0
)

type GoBu struct {
	ldflags     string
	buildflags  []string
	environment map[string]string
	subcmd      string
}

func (g *GoBu) AddLdFlags(flags ...string) {

}

func (g *GoBu) AddBuildFlags(flags ...string) {

}

func (g *GoBu) SetEnv(key, value string) {

}

func (g *GoBu) Getcmd() (command []string, env map[string]string) {
	return nil, nil
}

func fault(err error, message string, arg ...string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s%s: %s\n", message, strings.Join(arg, " "), err)
		exitValue = 1
		os.Exit(exitValue)
	}
}

func main() {
	opts := util.NewOptions()

	opts.Set("program-name", os.Args[0])
	opts.Set("program-version", progVersion)
	opts.Set("program-timestamp", timestamp)

	rep, err := git.PlainOpen(".")
	fault(err, "repo open")

	wt, err := rep.Worktree()
	fault(err, "worktree open")

	st, err := wt.Status()
	fault(err, "worktree status")
	fmt.Println(st.String())
}
