package main

//go:generate licrep -o licenses.go

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

	"github.com/kopoli/appkit"
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
	gcflags    []string
	environ    []string
	givenOs    string
	version    string
	binary     string
	subcmd     string
	name       string
	dopackage  bool
}

func (g *gobu) AddLdFlags(flags ...string) {
	g.ldflags = append(g.ldflags, flags...)
}

func (g *gobu) ResetLdFlags() {
	g.ldflags = nil
}

func (g *gobu) AddVar(name, value string) {
	g.AddLdFlags("-X", fmt.Sprintf("%s=%s", name, value))
}

func (g *gobu) AddBuildFlags(flags ...string) {
	g.buildflags = append(g.buildflags, flags...)
}

func (g *gobu) ResetBuildFlags() {
	g.buildflags = nil
}

func (g *gobu) AddCompileFlags(flags ...string) {
	g.gcflags = append(g.gcflags, flags...)
}

func (g *gobu) ResetCompileFlags() {
	g.gcflags = nil
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
	if g.binary == "" {
		g.binary = "go"
	}
	if g.subcmd == "" {
		g.subcmd = "build"
	}
	command = append(command, g.binary, g.subcmd)

	if g.buildflags != nil {
		command = append(command, g.buildflags...)
	}

	if g.ldflags != nil {
		command = append(command, "-ldflags", strings.Join(g.ldflags, " "))
	}

	if g.gcflags != nil {
		command = append(command, "-gcflags", strings.Join(g.gcflags, " "))
	}

	return command, g.environ
}

func (g *gobu) getTransformedBinaryName(name string) string {
	if g.name != "" {
		return strings.ReplaceAll(g.name, "%n", name)
	}
	return name
}

func (g *gobu) getBinaryName() (string, error) {
	archive, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "", err
	}
	return g.getTransformedBinaryName(filepath.Base(archive)), nil
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

	binary, err := g.getBinaryName()
	if err != nil {
		return err
	}
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
	help       string
	trait      func()
	paramTrait func(string)
}

type descmap map[string]traitdesc

func (d *descmap) add(name, help string, trait func()) {
	(*d)[name] = traitdesc{
		help:       help,
		trait:      trait,
		paramTrait: nil,
	}
}

func (d *descmap) addFlag(name, help string, trait func(string)) {
	(*d)[name] = traitdesc{
		help:       help,
		trait:      nil,
		paramTrait: trait,
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
	t.add("race", "Set '-race' build flag.", func() {
		gb.AddBuildFlags("-race")
	})
	t.add("rebuild", "Set '-a' build flag.", func() {
		gb.AddBuildFlags("-a")
	})
	t.add("trimpath", "Set '-trimpath' build flag.", func() {
		gb.AddBuildFlags("-trimpath")
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
	t.add("release", "Sets the traits: shrink, version, static, rebuild and trimpath.", func() {
		ret.apply("shrink", "version", "static", "rebuild", "trimpath")
	})
	t.add("default", "Sets the version trait. This is used if run without arguments.", func() {
		ret.apply("version")
	})

	t.addFlag("go=", "Set the 'go' binary explicitly.", func(s string) {
		gb.binary = s
	})
	t.addFlag("ldflags=", "Set 'go tool link' flags explicitly.", func(s string) {
		gb.ResetLdFlags()
		gb.AddLdFlags(s)
	})
	t.addFlag("buildflags=", "Set 'go build' flags explicitly.", func(s string) {
		gb.ResetBuildFlags()
		gb.AddBuildFlags(s)
	})
	t.addFlag("gcflags=", "Set 'go tool compile' flags explicitly.", func(s string) {
		gb.ResetCompileFlags()
		gb.AddCompileFlags(s)
	})
	t.addFlag("name=", "Set binary name with the -o build flag. %n represents original name.", func(s string) {
		gb.name = s
		name, err := gb.getBinaryName()
		if err != nil {
			panic(err)
		}
		gb.AddBuildFlags("-o", name)
	})
	ret.traits = t

	return ret
}

func isFlagTrait(name string) bool {
	return strings.Index(name, "=") != -1
}

func parseTrait(name string) string {
	return strings.SplitAfter(name, "=")[0]
}

func (g *gobutraits) check(names ...string) error {
	inv := make(map[string]bool)

	for i := range names {
		n := parseTrait(names[i])
		if _, ok := g.traits[n]; !ok {
			inv[n] = true
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
		n := parseTrait(names[i])
		if _, ok := g.applied[n]; ok {
			continue
		}
		if t, ok := g.traits[n]; ok {
			if isFlagTrait(n) {
				t.paramTrait(strings.SplitN(names[i], "=", 2)[1])
			} else {
				t.trait()
			}
			g.applied[n] = true
		}
	}
}

func (g *gobutraits) appliedTraits() []string {
	var ret []string

	for k, v := range g.applied {
		if v {
			ret = append(ret, k)
		}
	}

	return ret
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
var optDryRun = flag.Bool("dryrun", false, "Don't actually run any commands. Implies '-d'.")
var optLicenses = flag.Bool("licenses", false, "Show licenses of gobu.")

func main() {
	opts := appkit.NewOptions()
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
		fmt.Println(appkit.VersionString(opts))
		os.Exit(0)
	}

	if *optLicenses {
		l, err := GetLicenses()
		fault(err, "Getting licenses failed")
		s, err := appkit.LicenseString(l)
		fault(err, "Interpreting licenses failed")
		fmt.Print(s)
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

		wr := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(wr, "Traits:")
		printTrait := func(i int) {
			fmt.Fprintf(wr, "  %s\t%s\n", names[i], tr.traits[names[i]].help)
		}
		for i := range names {
			if !isFlagTrait(names[i]) {
				printTrait(i)
			}
		}
		fmt.Fprintln(wr, "\nParameterized traits:")
		for i := range names {
			if isFlagTrait(names[i]) {
				printTrait(i)
			}
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

	if *optDebug || *optDryRun {
		fmt.Printf("Traits:\n%s\nCommand:\n%s\nEnvironment:\n%s\n",
			strings.Join(tr.appliedTraits(), " "),
			strings.Join(c, " "), strings.Join(e, "\n"))
	}

	if *optDryRun {
		os.Exit(0)
	}

	err = runCommand(c, e)
	fault(err, "Build failed")

	if gb.dopackage {
		err = gb.createPackage()
		fault(err, "Creating package failed")
	}

	os.Exit(0)
}
