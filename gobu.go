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
	majorVersion       = "0"
	version            = "Undefined"
	timestamp          = "Undefined"
	buildGOOS          = "Undefined"
	buildGOARCH        = "Undefined"
	progVersion        = majorVersion + "-" + version
	exitValue    int   = 0
	gb           *GoBu = &GoBu{}
)

type GoBu struct {
	ldflags    []string
	buildflags []string
	environ    []string
	givenOs    string
	version    string
	subcmd     string
	dopackage  bool
}

func (g *GoBu) AddLdFlags(flags ...string) {
	g.ldflags = append(g.ldflags, flags...)
}

func (g *GoBu) AddVar(name, value string) {
	g.AddLdFlags("-X", fmt.Sprintf("%s=%s", name, value))
}

func (g *GoBu) AddBuildFlags(flags ...string) {
	g.buildflags = append(g.buildflags, flags...)
}

func (g *GoBu) SetEnv(key, value string) {
	g.environ = append(g.environ, fmt.Sprintf("%s=%s", key, value))
	if key == "GOOS" {
		g.givenOs = value
	}
}

func (g *GoBu) TargetOs() string {
	if g.givenOs != "" {
		return g.givenOs
	}
	return runtime.GOOS
}

func (g *GoBu) Getcmd() (command []string, env []string) {
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

func CreatePackage() error {
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
		progname = fmt.Sprintf("%s-%s", progname, gb.version)
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
	defer fp.Close()

	w := zip.NewWriter(fp)
	defer w.Close()

	properfiles := []string{}
	for i := range files {
		f, err := filepath.Glob(files[i])
		if err != nil || len(f) == 0 {
			continue
		}

		properfiles = append(properfiles, f...)
	}
	files = properfiles

	for i := range files {
		fw, err := w.Create(fmt.Sprintf("%s/%s", progname, files[i]))
		if err != nil {
			return err
		}
		rfp, err := os.Open(files[i])
		if err != nil {
			return err
		}

		_, err = io.Copy(fw, rfp)
		if err != nil {
			return err
		}
	}

	return nil
}

var Traits = map[string]func(){
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
}
var appliedTraits = map[string]bool{}

func applyTraits(traits ...string) {
	for i := range traits {
		if _, ok := appliedTraits[traits[i]]; ok {
			continue
		}
		if t, ok := Traits[traits[i]]; ok {
			t()
			appliedTraits[traits[i]] = true
		}
	}
}

func runCommand(args []string, env []string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
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
		fmt.Fprintf(os.Stderr, "Error: %s%s: %s\n", message, strings.Join(arg, " "), err)
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
	Traits["release"] = func() {
		applyTraits("shrink", "version", "static", "rebuild")
	}
	Traits["default"] = func() {
		applyTraits("version")
	}

	if *optListTraits {
		traits := []string{}
		for k := range Traits {
			traits = append(traits, k)
		}
		sort.Strings(traits)
		fmt.Println("Traits:")
		for i := range traits {
			fmt.Println("  ", traits[i])
		}
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		args = []string{"default"}
	}

	applyTraits(args...)
	c, e := gb.Getcmd()

	if *optDebug {
		fmt.Printf("Command: %v\nEnvironment: %v", c, e)
	}

	err := runCommand(c, e)
	fault(err, "Build failed")

	if gb.dopackage {
		err = CreatePackage()
		fault(err, "Creating package failed")
	}

	os.Exit(0)
}
