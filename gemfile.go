package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

type Gemfile struct {
	Gems []Gem
}

var topicRegexp = regexp.MustCompile("^[A-Z]+$")
var gitRemoteRegexp = regexp.MustCompile("^  remote: (.+)$")
var gitRevisionRegexp = regexp.MustCompile("^  revision: ([0-9a-f]+)$")
var gemRegexp = regexp.MustCompile("^    ([A-Za-z0-9\\.\\-\\_]+?) \\(([^)]+)\\)$")
var extensionRegexp = regexp.MustCompile("\n  s.executables = \\[(.+)\\]\n")

func ReadGemfileLock(filename string) (gf *Gemfile, err error) {
	if _, err = os.Stat(filename); os.IsNotExist(err) {
		return
	}

	fd, err := os.Open(filename)
	defer fd.Close()
	if err != nil {
		return
	}

	gf = new(Gemfile)

	topic := ""
	remote := ""
	revision := ""

	scanner := bufio.NewScanner(fd)

	for scanner.Scan() {
		if topicRegexp.Match(scanner.Bytes()) {
			topic = scanner.Text()
		}
		switch topic {
		case "GIT":
			if parts := gitRemoteRegexp.FindStringSubmatch(scanner.Text()); len(parts) > 0 {
				remote = parts[1]
			}
			if parts := gitRevisionRegexp.FindStringSubmatch(scanner.Text()); len(parts) > 0 {
				revision = parts[1]
			}
			if parts := gemRegexp.FindStringSubmatch(scanner.Text()); len(parts) > 0 {
				gem := new(GitGem)
				gem.Name = parts[1]
				gem.Version = parts[2]
				gem.Remote = remote
				gem.Commit = revision
				gf.Gems = append(gf.Gems, gem)
			}
		case "GEM":
			if parts := gitRemoteRegexp.FindStringSubmatch(scanner.Text()); len(parts) > 0 {
				remote = parts[1]
			}
			if parts := gemRegexp.FindStringSubmatch(scanner.Text()); len(parts) > 0 {
				gem := new(IndexGem)
				gem.Name = parts[1]
				gem.Version = parts[2]
				gem.Remote = remote
				gf.Gems = append(gf.Gems, gem)
			}
		}
	}

	return
}

func (gf *Gemfile) Install(root string) (err error) {
	ch := make(chan Gem)
	dir := gf.installDir(root)
	for _, gem := range gf.Gems {
		go func(gem Gem) {
			gem.Install(dir)
			ch <- gem
		}(gem)
	}
	for i := 0; i < len(gf.Gems); i++ {
		gem := <-ch
		switch(gem.InstallState()) {
		case INSTALLED:
			fmt.Printf("Installed %s\n", gem.Banner())
		case SKIPPED:
			fmt.Printf("Skipped %s\n", gem.Banner())
		}
	}
	Execute(Cmd{ Command:fmt.Sprintf("bundle config --local path %s", root) })
	return
}

func (gf *Gemfile) apiVersion() (version string) {
	buffer := new(bytes.Buffer)
	Execute(Cmd{ Command:"ruby -e 'print Gem::ConfigMap[:ruby_version]'", Output:buffer })
	return buffer.String()
}

func (gf *Gemfile) installDir(root string) (dir string) {
	dir, _ = filepath.Abs(filepath.Join(root, "ruby", gf.apiVersion()))
	return
}
