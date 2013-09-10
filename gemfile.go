package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type Gem interface {
	Install(string, string) error
	Banner() string
}

type GitGem struct {
	Name    string
	Version string
	Remote  string
	Commit  string
}

type IndexGem struct {
	Name    string
	Version string
	Remote  string
}

type Gemfile struct {
	Gems []Gem
	Api  string
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

	gf.Api = executeCommandOutput(".", "ruby -e 'print Gem::ConfigMap[:ruby_version]'")

	return
}

func (gf *Gemfile) Install(home string) (err error) {
	ch := make(chan Gem)
	for _, gem := range gf.Gems {
		go func(gem Gem) {
			gem.Install(home, gf.Api)
			ch <- gem
		}(gem)
	}
	for i := 0; i < len(gf.Gems); i++ {
		gem := <-ch
		fmt.Printf("Installed %s\n", gem.Banner())
	}
	executeCommand(".", fmt.Sprintf("bundle config --local path %s", home))
	return
}

func (gem *GitGem) Install(home string, api string) (err error) {
	commit := gem.Commit[0:12]
	target := filepath.Join(home, fmt.Sprintf("ruby/%s/bundler/gems/%s-%s", api, gem.Name, commit))
	os.RemoveAll(target)
	os.MkdirAll(target, 0755)
	executeCommand(target, "git init")
	executeCommand(target, fmt.Sprintf("git remote add origin %s", gem.Remote))
	executeCommand(target, "git fetch origin")
	executeCommand(target, fmt.Sprintf("git checkout %s", gem.Commit))
	return
}

func (gem *IndexGem) Install(home string, api string) (err error) {
	cache_target := filepath.Join(home, fmt.Sprintf("ruby/%s/cache", api))
	cache_file, _ := filepath.Abs(filepath.Join(cache_target, fmt.Sprintf("%s-%s.gem", gem.Name, gem.Version)))
	gems_target := filepath.Join(home, fmt.Sprintf("ruby/%s/gems/%s-%s", api, gem.Name, gem.Version))
	spec_target := filepath.Join(home, fmt.Sprintf("ruby/%s/specifications", api))
	spec_file, _ := filepath.Abs(filepath.Join(spec_target, fmt.Sprintf("%s-%s.gemspec", gem.Name, gem.Version)))
	bin_target := filepath.Join(home, fmt.Sprintf("ruby/%s/bin", api))
	url := fmt.Sprintf("%sgems/%s-%s.gem", gem.Remote, gem.Name, gem.Version)
	os.MkdirAll(cache_target, 0755)
	os.Remove(cache_file)
	executeCommand(cache_target, fmt.Sprintf("curl -L -o %s %s", cache_file, url))
	os.RemoveAll(gems_target)
	os.MkdirAll(gems_target, 0755)
	executeCommand(gems_target, fmt.Sprintf("tar -xzvf %s", cache_file))
	executeCommand(gems_target, "tar -xzvf data.tar.gz && rm data.tar.gz")
	executeCommand(gems_target, "gzip -d metadata.gz")
	os.MkdirAll(spec_target, 0755)
	executeCommand(gems_target, fmt.Sprintf("gem specification %s --ruby > %s", cache_file, spec_file))
	bytes, _ := ioutil.ReadFile(filepath.Join(gems_target, "metadata"))
	metadata := string(bytes)
	gem_home, _ := filepath.Abs(filepath.Join(home, fmt.Sprintf("ruby/%s", api)))
	if !strings.Contains(metadata, "extensions: []") {
		executeCommand(gems_target, fmt.Sprintf("env GEM_HOME=%s ruby -e 'require \"rubygems/installer\"; Gem::Installer.new(\"../../cache/%s-%s.gem\").build_extensions'", gem_home, gem.Name, gem.Version))
	}
	os.MkdirAll(bin_target, 0755)
	bytes, _ = ioutil.ReadFile(spec_file)
	if parts := extensionRegexp.FindStringSubmatch(string(bytes)); len(parts) > 0 {
		for _, binary := range strings.Split(strings.Replace(strings.Replace(parts[1], " ", "", -1), "\"", "", -1), ",") {
			bin_file, _ := filepath.Abs(filepath.Join(bin_target, binary))
			writeBinstub(bin_file, gem.Name, binary)
		}
	}
	return
}

func (gem *GitGem) Banner() string {
	return fmt.Sprintf("%s (%s) from %s#%s", gem.Name, gem.Version, gem.Remote, gem.Commit[0:12])
}

func (gem *IndexGem) Banner() string {
	return fmt.Sprintf("%s (%s) from %s", gem.Name, gem.Version, gem.Remote)
}

func executeCommand(dir string, command string) {
	cmd := []string{"/bin/bash", "-c", command}
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Dir = dir
	/* c.Stdout = os.Stdout*/
	/* c.Stderr = os.Stderr*/
	c.Start()
	c.Wait()
	if !c.ProcessState.Success() {
		fmt.Printf("ERROR: %s\n", command)
		os.Exit(2)
	}
}

func executeCommandOutput(dir string, command string) string {
	buffer := new(bytes.Buffer)
	cmd := []string{"/bin/bash", "-c", command}
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Dir = dir
	c.Stdout = buffer
	c.Start()
	c.Wait()
	return buffer.String()
}

func writeBinstub(filename string, gem string, binary string) {
	os.Remove(filename)
	fd, _ := os.Create(filename)
	fd.WriteString(fmt.Sprintf(`#!/usr/bin/env ruby
#
# This file was generated by RubyGems.
#
# The application 'sass' is installed as part of a gem, and
# this file is here to facilitate running it.
#

require 'rubygems'

version = ">= 0"

if ARGV.first =~ /^_(.*)_$/ and Gem::Version.correct? $1 then
  version = $1
  ARGV.shift
end

gem 'sass', version
load Gem.bin_path('%s', '%s', version)`, gem, binary))
	fd.Close()
	os.Chmod(filename, 0755)
}
