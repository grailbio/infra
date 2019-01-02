[![API Reference](https://img.shields.io/badge/api-reference-blue.svg)](https://godoc.org/github.com/grailbio/infra) 

# Infra

Package infra provides cloud infrastructure management for Go programs.
The package includes facilities for configuring, provisioning, and
migrating cloud infrastructure that is used by a Go program. You can
think of infra as a simple, embedded version of Terraform, combined with
a self-contained dependency injection framework.

Infrastructure managed by this package is exposed through a
configuration. Configurations specify which providers should be used for
which infrastructure component; configurations also store provider
configuration and migration state. Users instantiate typed values
directly from the configuration: the details of configuration and of
managing dependencies between infrastructure components is handled by
the config object itself. Configurations are marshaled and must be
stored by the user.

Infrastructure migration is handled by maintaining a set of versions for
each provider; migrations perform side-effects and can modify the
configuration accordingly (e.g., to store identifiers used by the cloud
infrastructure provider).

