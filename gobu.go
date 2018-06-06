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
	"text/tabwriter"
	"time"

	util "github.com/kopoli/go-util"
)

var (
	version     = "Undefined"
	timestamp   = "Undefined"
	buildGOOS   = "Undefined"
	buildGOARCH = "Undefined"
	progVersion = "" + version
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
func (g *gobu) createPackage() error {
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
	if g.version != "" {
		progname = fmt.Sprintf("%s-%s-%s-%s", progname, g.version,
			g.TargetOs(), runtime.GOARCH)
	}
	zipfile := fmt.Sprintf("%s.zip", progname)

	if g.TargetOs() == "windows" {
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

type traitdesc struct {
	help  string
	trait func()
}

type descmap map[string]traitdesc

func (d *descmap) add(name, help string, trait func()) {
	(*d)[name] = traitdesc{
		help:  help,
		trait: trait,
	}
}

type gobutraits struct {
	traits  descmap
	applied map[string]bool
}

func newgobutraits(gb *gobu) *gobutraits {
	var ret = &gobutraits{
		applied: make(map[string]bool),
	}
	t := make(descmap)

	t.add("nocgo", "Set 'CGO_ENABLED=0' environment variable.", func() {
		gb.SetEnv("CGO_ENABLED", "0")
	})
	t.add("static", "Set '-extldflags \"-static\"' link flags.", func() {
		gb.AddLdFlags("-extldflags", `"-static"`)
	})
	t.add("shrink", "Set '-s -w' link flags.", func() {
		gb.AddLdFlags("-s", "-w")
	})
	t.add("rebuild", "Set '-a' build flag.", func() {
		gb.AddBuildFlags("-a")
	})
	t.add("linux", "Set 'GOOS=linux' environment variable.", func() {
		gb.SetEnv("GOOS", "linux")
	})
	t.add("windows", "Set 'GOOS=windows' environment variable.", func() {
		gb.SetEnv("GOOS", "windows")
	})
	t.add("windowsgui", "Set windows trait and '-H windowsgui' link flag.", func() {
		ret.apply("windows")
		gb.AddLdFlags("-H", "windowsgui")
	})
	t.add("verbose", "Set '-v' build flag.", func() {
		gb.AddBuildFlags("-v")
	})
	t.add("debug", "Set '-x' build flag.", func() {
		gb.AddBuildFlags("-x")
	})
	t.add("install", "Run 'go install' instead of 'go build'.", func() {
		gb.subcmd = "install"
	})
	t.add("version", "Set 'timestamp', 'version', 'buildGOOS' and 'buildGOARCH' go variables to the 'main' package.", func() {
		gb.AddVar("main.timestamp", time.Now().Format(time.RFC3339))
		gb.AddVar("main.version", gb.version)
		gb.AddVar("main.buildGOOS", runtime.GOOS)
		gb.AddVar("main.buildGOARCH", runtime.GOARCH)
	})
	t.add("package", "After building creates a zip-package of the binary.", func() {
		gb.dopackage = true
	})
	t.add("release", "Sets the traits: shrink, version, static and rebuild.", func() {
		ret.apply("shrink", "version", "static", "rebuild")
	})
	t.add("default", "Sets the version trait. This is used if run without arguments.", func() {
		ret.apply("version")
	})

	ret.traits = t

	return ret
}

func (g *gobutraits) check(names ...string) error {
	inv := make(map[string]bool)

	for i := range names {
		if _, ok := g.traits[names[i]]; !ok {
			inv[names[i]] = true
		}
	}

	suffix := "s"
	switch len(inv) {
	case 0:
		return nil
	case 1:
		suffix = ""
	}

	var invalid []string
	for k := range inv {
		invalid = append(invalid, k)
	}

	return fmt.Errorf("Invalid trait%s: %s", suffix, strings.Join(invalid, ", "))
}

func (g *gobutraits) apply(names ...string) {
	for i := range names {
		if _, ok := g.applied[names[i]]; ok {
			continue
		}
		if t, ok := g.traits[names[i]]; ok {
			t.trait()
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
		os.Exit(1)
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

	gb := &gobu{
		version: cmdStr("git", "describe", "--always", "--tags", "--dirty"),
	}

	tr := newgobutraits(gb)

	if *optListTraits {
		names := []string{}
		for k := range tr.traits {
			names = append(names, k)
		}
		sort.Strings(names)
		fmt.Println("Traits:")
		wr := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		for i := range names {
			fmt.Fprintf(wr, "  %s\t%s\n", names[i], tr.traits[names[i]].help)
		}
		wr.Flush()
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		args = []string{"default"}
	}

	err := tr.check(args...)
	fault(err, "Parsing command line failed")

	tr.apply(args...)
	c, e := gb.Getcmd()

	if *optDebug {
		fmt.Printf("Command: %s\nEnvironment:\n%s\n",
			strings.Join(c, " "), strings.Join(e, "\n"))
	}

	err = runCommand(c, e)
	fault(err, "Build failed")

	if gb.dopackage {
		err = gb.createPackage()
		fault(err, "Creating package failed")
	}

	os.Exit(0)
}
