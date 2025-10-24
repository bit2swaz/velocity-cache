package velocitycache

import _ "embed"

// DefaultVelocityConfig contains the default velocity configuration template.
//go:embed velocity.config.json.example
var DefaultVelocityConfig []byte

// VelocityConfigTemplate returns a safe copy of the default configuration template.
func VelocityConfigTemplate() []byte {
	buf := make([]byte, len(DefaultVelocityConfig))
	copy(buf, DefaultVelocityConfig)
	return buf
}
