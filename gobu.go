package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	util "github.com/kopoli/go-util"
)

var (
	majorVersion = "0"
	version      = "Undefined"
	timestamp    = "Undefined"
	buildGOOS    = "Undefined"
	buildGOARCH  = "Undefined"
	progVersion  = majorVersion + "-" + version
	exitValue    int
	gb           = &gobu{}
)

type gobu struct {
	ldflags    []string
	buildflags []string
	environ    []string
	givenOs    string
	version    string
	subcmd     string
	dopackage  bool
}

func (g *gobu) AddLdFlags(flags ...string) {
	g.ldflags = append(g.ldflags, flags...)
}

func (g *gobu) AddVar(name, value string) {
	g.AddLdFlags("-X", fmt.Sprintf("%s=%s", name, value))
}

func (g *gobu) AddBuildFlags(flags ...string) {
	g.buildflags = append(g.buildflags, flags...)
}

func (g *gobu) SetEnv(key, value string) {
	g.environ = append(g.environ, fmt.Sprintf("%s=%s", key, value))
	if key == "GOOS" {
		g.givenOs = value
	}
	err := os.Setenv(key, value)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Error: Failed to set environment variable %s=%s: %s",
			key, value, err)
	}
}

func (g *gobu) TargetOs() string {
	if g.givenOs != "" {
		return g.givenOs
	}
	return runtime.GOOS
}

func (g *gobu) Getcmd() (command []string, env []string) {
	if g.subcmd == "" {
		g.subcmd = "build"
	}
	command = append(command, "go", g.subcmd)

	if g.buildflags != nil {
		command = append(command, g.buildflags...)
	}

	if g.ldflags != nil {
		command = append(command, "-ldflags", strings.Join(g.ldflags, " "))
	}

	return command, g.environ
}

// createPackage creates a zip package of the built binary and some extra
// files. The environment variable GOBU_EXTRA_DIST can be used to include
// additional files to the zip package.
func createPackage() error {
	var err error
	filestr := os.Getenv("GOBU_EXTRA_DIST")
	files := []string{"README*", "LICENSE"}
	if filestr != "" {
		files = strings.Split(filestr, " ")
	}

	archive, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return err
	}
	binary := filepath.Base(archive)
	progname := binary
	if gb.version != "" {
		progname = fmt.Sprintf("%s-%s-%s-%s", progname, gb.version,
			gb.TargetOs(), runtime.GOARCH)
	}
	zipfile := fmt.Sprintf("%s.zip", progname)

	if gb.TargetOs() == "windows" {
		binary = binary + ".exe"
	}
	files = append(files, binary)

	fp, err := os.Create(zipfile)
	if err != nil {
		return err
	}
	defer func() {
		e2 := fp.Close()
		if err == nil && e2 != nil {
			err = e2
		}
	}()

	w := zip.NewWriter(fp)
	defer func() {
		e2 := w.Close()
		if err == nil && e2 != nil {
			err = e2
		}
	}()

	properfiles := []string{}
	for i := range files {
		var f []string
		f, err = filepath.Glob(files[i])
		if err != nil || len(f) == 0 {
			continue
		}

		properfiles = append(properfiles, f...)
	}
	files = properfiles

	for i := range files {
		var fw io.Writer
		fw, err = w.Create(fmt.Sprintf("%s/%s", progname, files[i]))
		if err != nil {
			return err
		}
		var rfp *os.File
		rfp, err = os.Open(files[i])
		if err != nil {
			return err
		}

		_, err = io.Copy(fw, rfp)
		if err != nil {
			return err
		}
	}

	return err
}

type gobutraits struct {
	traits  map[string]func()
	applied map[string]bool
}

func newgobutraits() *gobutraits {
	var ret = &gobutraits{
		applied: make(map[string]bool),
	}
	ret.traits = map[string]func(){
		"nocgo": func() {
			gb.SetEnv("CGO_ENABLED", "0")
		},
		"static": func() {
			gb.AddLdFlags("-extldflags", `"-static"`)
		},
		"shrink": func() {
			gb.AddLdFlags("-s", "-w")
		},
		"rebuild": func() {
			gb.AddBuildFlags("-a")
		},
		"linux": func() {
			gb.SetEnv("GOOS", "linux")
		},
		"windows": func() {
			gb.SetEnv("GOOS", "windows")
		},
		"windowsgui": func() {
			ret.apply("windows")
			gb.AddLdFlags("-H", "windowsgui")
		},
		"verbose": func() {
			gb.AddBuildFlags("-v")
		},
		"debug": func() {
			gb.AddBuildFlags("-x")
		},
		"install": func() {
			gb.subcmd = "install"
		},
		"version": func() {
			gb.AddVar("main.timestamp", time.Now().Format(time.RFC3339))
			gb.AddVar("main.version", gb.version)
			gb.AddVar("main.buildGOOS", runtime.GOOS)
			gb.AddVar("main.buildGOARCH", runtime.GOARCH)
		},
		"package": func() {
			gb.dopackage = true
		},
		"release": func() {
			ret.apply("shrink", "version", "static", "rebuild")
		},
		"default": func() {
			ret.apply("version")
		},
	}

	return ret
}

func (g *gobutraits) apply(names ...string) {
	for i := range names {
		if _, ok := g.applied[names[i]]; ok {
			continue
		}
		if t, ok := g.traits[names[i]]; ok {
			t()
			g.applied[names[i]] = true
		}
	}
}

func runCommand(args []string, env []string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func cmdStr(args ...string) string {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.Trim(string(out), " \n\r\t")
}

func fault(err error, message string, arg ...string) {
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %s%s: %s\n", message, strings.Join(arg, " "), err)
		exitValue = 1
		os.Exit(exitValue)
	}
}

var optVersion = flag.Bool("v", false, "Display version")
var optListTraits = flag.Bool("l", false, "List traits")
var optDebug = flag.Bool("d", false, "Enable debug output")

func main() {
	opts := util.NewOptions()
	opts.Set("program-name", os.Args[0])
	opts.Set("program-version", progVersion)
	opts.Set("program-timestamp", timestamp)
	opts.Set("program-buildgoos", buildGOOS)
	opts.Set("program-buildgoarch", buildGOARCH)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s: Traitful go build\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Command line options:")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *optVersion {
		fmt.Println(util.VersionString(opts))
		os.Exit(0)
	}

	gb.version = cmdStr("git", "describe", "--always", "--tags", "--dirty")

	tr := newgobutraits()

	if *optListTraits {
		names := []string{}
		for k := range tr.traits {
			names = append(names, k)
		}
		sort.Strings(names)
		fmt.Println("Traits:")
		for i := range names {
			fmt.Println("  ", names[i])
		}
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		args = []string{"default"}
	}

	tr.apply(args...)
	c, e := gb.Getcmd()

	if *optDebug {
		fmt.Printf("Command: %s\nEnvironment:\n%s\n",
			strings.Join(c, " "), strings.Join(e, "\n"))
	}

	err := runCommand(c, e)
	fault(err, "Build failed")

	if gb.dopackage {
		err = createPackage()
		fault(err, "Creating package failed")
	}

	os.Exit(0)
}
