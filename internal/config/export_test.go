package config

// NameLooksSecret exposes the internal name heuristic to the external test
// package so TestSecretsAreExplicit can use it as a lower bound on EnvVar.Secret.
var NameLooksSecret = nameLooksSecret
