package ticket

import "fmt"

// IDStrategy controls how ticket IDs are generated.
type IDStrategy string

const (
	// IDStrategyDefault uses goname's default word-list strategy.
	IDStrategyDefault IDStrategy = "default"
	// IDStrategyHex generates hexadecimal ticket IDs.
	IDStrategyHex IDStrategy = "hex"
	// IDStrategyTolkien generates Tolkien-themed word ticket IDs.
	IDStrategyTolkien IDStrategy = "tolkien"
	// IDStrategyBase32 generates Crockford Base32 ticket IDs.
	IDStrategyBase32 IDStrategy = "base32"
	// IDStrategyULID generates ULID ticket IDs.
	IDStrategyULID IDStrategy = "ulid"
)

// ParseIDStrategy validates and normalizes a configured or command-line ID
// strategy.
func ParseIDStrategy(value string) (IDStrategy, error) {
	switch IDStrategy(value) {
	case IDStrategyDefault, IDStrategyHex, IDStrategyTolkien, IDStrategyBase32, IDStrategyULID:
		return IDStrategy(value), nil
	default:
		return "", fmt.Errorf("invalid ID strategy %q (valid values: default|tolkien|hex|base32|ulid)", value)
	}
}
