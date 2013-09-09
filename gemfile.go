package main

import (
  "bufio"
  "fmt"
  "os"
  "os/exec"
  "path/filepath"
  "regexp"
)

type Gem interface {
  Install(string) error
  Banner() string
}

type GitGem struct {
  Name string
  Version string
  Remote string
  Commit string
}

type IndexGem struct {
  Name string
  Version string
  Remote string
}

type Gemfile struct {
  Gems []Gem
}

var topicRegexp = regexp.MustCompile("^[A-Z]+$")
var gitRemoteRegexp = regexp.MustCompile("^  remote: (.+)$")
var gitRevisionRegexp = regexp.MustCompile("^  revision: ([0-9a-f]+)$")
var gemRegexp = regexp.MustCompile("^    ([A-Za-z0-9\\.\\-\\_]+?) \\(([^)]+)\\)$")

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
    switch (topic) {
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

func (gf *Gemfile) Install(home string) (err error) {
  ch := make(chan Gem)
  for _, gem := range gf.Gems {
    go func(gem Gem) {
      gem.Install(home)
      ch <- gem
    }(gem)
  }
  for i := 0; i < len(gf.Gems); i++ {
    gem := <-ch
    fmt.Printf("Installed %s\n", gem.Banner())
  }
  return
}

func (gem *GitGem) Install(home string) (err error) {
  commit := gem.Commit[0:12]
  target := filepath.Join(home, fmt.Sprintf("ruby/1.9.1/bundler/gems/%s-%s", gem.Name, commit))
  os.RemoveAll(target)
  os.MkdirAll(target, 0755)
  executeCommand(target, "git init")
  executeCommand(target, fmt.Sprintf("git remote add origin %s", gem.Remote))
  executeCommand(target, "git fetch origin")
  executeCommand(target, fmt.Sprintf("git checkout %s", gem.Commit))
  return
}

func (gem *IndexGem) Install(home string) (err error) {
  target := filepath.Join(home, fmt.Sprintf("ruby/1.9.1/gems/%s-%s", gem.Name, gem.Version))
  url := fmt.Sprintf("%sgems/%s-%s.gem", gem.Remote, gem.Name, gem.Version)
  os.RemoveAll(target)
  os.MkdirAll(target, 0755)
  executeCommand(target, fmt.Sprintf("curl -L %s | tar -xzvf -", url))
  executeCommand(target, "tar -xzvf data.tar.gz")
  executeCommand(target, "rm data.tar.gz metadata.gz")
  return
}

func (gem *GitGem) Banner() (string) {
  return fmt.Sprintf("%s (%s) from %s#%s", gem.Name, gem.Version, gem.Remote, gem.Commit[0:12])
}

func (gem *IndexGem) Banner() (string) {
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
}
