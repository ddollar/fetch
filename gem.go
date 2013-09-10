package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type GemInstallState int

const (
	INSTALLED GemInstallState = iota
	SKIPPED
)

type Gem interface {
	Banner() string
	Install(root string)
	InstallState() GemInstallState
}

type GitGem struct {
	Name    string
	Version string
	Remote  string
	Commit  string
	State   GemInstallState
}

type IndexGem struct {
	Name    string
	Version string
	Remote  string
	State   GemInstallState
}

/** GitGem ******************************************************************/

func (gem *GitGem) Banner() string {
	return fmt.Sprintf("%s (%s) from %s#%s", gem.Name, gem.Version, gem.Remote, gem.Commit[0:12])
}

func (gem *GitGem) Install(root string) {
	if _, err := os.Stat(gem.installDir(root)); os.IsNotExist(err) {
		gem.install(root)
		gem.State = INSTALLED
	} else {
		gem.State = SKIPPED
	}
}

func (gem *GitGem) InstallState() GemInstallState {
	return gem.State
}

func (gem *GitGem) install(root string) {
	install_dir := gem.installDir(root)
	os.RemoveAll(install_dir)
	os.MkdirAll(install_dir, 0755)
	Execute(Cmd{Dir: install_dir, Command: "git init"})
	Execute(Cmd{Dir: install_dir, Command: fmt.Sprintf("git remote add origin %s", gem.Remote)})
	Execute(Cmd{Dir: install_dir, Command: "git fetch origin"})
	Execute(Cmd{Dir: install_dir, Command: fmt.Sprintf("git checkout %s", gem.Commit)})
}

func (gem *GitGem) createSpecification(root string) {
	install_dir := gem.installDir(root)
	spec_dir := gem.specificationDir(root)
	spec_file := gem.specificationFile(root)
	os.MkdirAll(spec_dir, 0755)
	os.Remove(spec_file)
	Execute(Cmd{Dir: install_dir, Command: fmt.Sprintf("ruby -e 'puts eval(File.read(\"%s.gemspec\")).to_ruby' > %s", gem.Name, spec_file)})
}

func (gem *GitGem) installDir(root string) (dir string) {
	dir, _ = filepath.Abs(filepath.Join(root, fmt.Sprintf("bundler/gems/%s-%s", gem.Name, gem.Commit[0:12])))
	return
}

func (gem *GitGem) specificationDir(root string) (dir string) {
	dir, _ = filepath.Abs(filepath.Join(root, "specifications"))
	return
}

func (gem *GitGem) specificationFile(root string) (file string) {
	file, _ = filepath.Abs(filepath.Join(gem.specificationDir(root), fmt.Sprintf("%s-%s.gemspec", gem.Name, gem.Version)))
	return
}

/** IndexGem ****************************************************************/

func (gem *IndexGem) Banner() string {
	return fmt.Sprintf("%s (%s) from %s", gem.Name, gem.Version, gem.Remote)
}

func (gem *IndexGem) Install(root string) {
	if _, err := os.Stat(gem.installDir(root)); os.IsNotExist(err) {
		gem.cache(root)
		gem.install(root)
		gem.createSpecification(root)
		gem.compileExtensions(root)
		gem.createBinstubs(root)
		gem.State = INSTALLED
	} else {
		gem.State = SKIPPED
	}
}

func (gem *IndexGem) InstallState() GemInstallState {
	return gem.State
}

func (gem *IndexGem) cache(root string) {
	cache_dir := gem.cacheDir(root)
	cache_file := gem.cacheFile(root)
	os.MkdirAll(cache_dir, 0755)
	os.Remove(cache_file)
	Execute(Cmd{Command: fmt.Sprintf("curl -L -o %s %s", cache_file, gem.remoteUrl())})
}

func (gem *IndexGem) install(root string) {
	install_dir := gem.installDir(root)
	os.RemoveAll(install_dir)
	os.MkdirAll(install_dir, 0755)
	Execute(Cmd{Dir: install_dir, Command: fmt.Sprintf("tar -xzvf %s", gem.cacheFile(root))})
	Execute(Cmd{Dir: install_dir, Command: "tar -xzvf data.tar.gz && rm data.tar.gz"})
	Execute(Cmd{Dir: install_dir, Command: "gzip -d metadata.gz"})
}

func (gem *IndexGem) createSpecification(root string) {
	cache_file := gem.cacheFile(root)
	spec_dir := gem.specificationDir(root)
	spec_file := gem.specificationFile(root)
	os.MkdirAll(spec_dir, 0755)
	os.Remove(spec_file)
	Execute(Cmd{Command: fmt.Sprintf("gem specification %s --ruby > %s", cache_file, spec_file)})
}

func (gem *IndexGem) compileExtensions(root string) {
	cache_file := gem.cacheFile(root)
	install_dir := gem.installDir(root)
	spec_file := gem.specificationFile(root)
	spec_bytes, _ := ioutil.ReadFile(spec_file)
	if !strings.Contains(string(spec_bytes), "extensions: []") {
		Execute(Cmd{Dir: install_dir, Command: fmt.Sprintf("env GEM_HOME=%s ruby -e 'require \"rubygems/installer\"; Gem::Installer.new(\"%s\").build_extensions'", root, cache_file)})
	}
}

func (gem *IndexGem) createBinstubs(root string) {
	bin_dir := gem.binDir(root)
	os.MkdirAll(bin_dir, 0755)
	buffer := new(bytes.Buffer)
	Execute(Cmd{Command: fmt.Sprintf("ruby -e 'puts eval(File.read(\"%s\")).executables'", gem.specificationFile(root)), Output: buffer})
	scanner := bufio.NewScanner(buffer)
	for scanner.Scan() {
		bin := scanner.Text()
		writeBinstub(gem.binFile(root, bin), gem.Name, bin)
	}
}

func (gem *IndexGem) binDir(root string) (dir string) {
	dir, _ = filepath.Abs(filepath.Join(root, "bin"))
	return
}

func (gem *IndexGem) binFile(root string, name string) (file string) {
	file, _ = filepath.Abs(filepath.Join(gem.binDir(root), name))
	return
}

func (gem *IndexGem) cacheDir(root string) (dir string) {
	dir, _ = filepath.Abs(filepath.Join(root, "cache"))
	return
}

func (gem *IndexGem) cacheFile(root string) (file string) {
	file, _ = filepath.Abs(filepath.Join(gem.cacheDir(root), fmt.Sprintf("%s-%s.gem", gem.Name, gem.Version)))
	return
}

func (gem *IndexGem) installDir(root string) (dir string) {
	dir, _ = filepath.Abs(filepath.Join(root, fmt.Sprintf("gems/%s-%s", gem.Name, gem.Version)))
	return
}

func (gem *IndexGem) remoteUrl() string {
	return fmt.Sprintf("%sgems/%s-%s.gem", gem.Remote, gem.Name, gem.Version)
}

func (gem *IndexGem) specificationDir(root string) (dir string) {
	dir, _ = filepath.Abs(filepath.Join(root, "specifications"))
	return
}

func (gem *IndexGem) specificationFile(root string) (file string) {
	file, _ = filepath.Abs(filepath.Join(gem.specificationDir(root), fmt.Sprintf("%s-%s.gemspec", gem.Name, gem.Version)))
	return
}

/** Common ******************************************************************/

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
