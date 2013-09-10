## fetch

Fetch is designed to install everything specified by a `Gemfile.lock` into a subdirectory called `vendor/bundle` and then configure Bundler to use that directory during runtime.

Fetch is not designed to be used during development or whenever your `Gemfile` is changing. Fetch is designed to be used once you have a `Gemfile.lock`, (e.g. during deploys to production)

Fetch will download and install the gems specified in your Gemfile.lock and then compile their binary extensions and create any specified binstubs.

### WARNING

#### FETCH IS HIGHLY EXPERIMENTAL AND MAY BREAK EVERYTHING YOU OWN

### Installation

##### Compile from Source

    $ go get -u github.com/ddollar/fetch

### Usage

    $ cd ~/my-ruby-app
    $ fetch
