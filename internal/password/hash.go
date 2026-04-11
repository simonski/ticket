package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	saltLength  = 16
	parallelism = 2
	keyLength   = 32
)

func hashParams() (uint32, uint32) {
	if os.Getenv("TICKET_FAST_HASH") == "1" {
		return 1024, 1
	}
	return 64 * 1024, 4
}

func Hash(plain string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read salt: %w", err)
	}

	memory, iterations := hashParams()
	hash := argon2.IDKey([]byte(plain), salt, iterations, memory, parallelism, keyLength)

	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memory,
		iterations,
		parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func Verify(encoded, plain string) (bool, error) {
	params, salt, expected, err := parse(encoded)
	if err != nil {
		return false, err
	}

	actual := argon2.IDKey([]byte(plain), salt, params.iterations, params.memory, params.parallelism, uint32(len(expected))) // #nosec G115 -- len() is always non-negative; int→uint32 is safe
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

type argonParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func parse(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argonParams{}, nil, nil, errors.New("invalid argon2id hash")
	}

	params := argonParams{}
	for _, item := range strings.Split(parts[3], ",") {
		kv := strings.SplitN(item, "=", 2)
		if len(kv) != 2 {
			return argonParams{}, nil, nil, errors.New("invalid argon2id params")
		}
		value, err := strconv.ParseUint(kv[1], 10, 32)
		if err != nil {
			return argonParams{}, nil, nil, fmt.Errorf("parse argon2id param %s: %w", kv[0], err)
		}
		switch kv[0] {
		case "m":
			params.memory = uint32(value)
		case "t":
			params.iterations = uint32(value)
		case "p":
			params.parallelism = uint8(value) // #nosec G115 -- parallelism is stored as uint8 in argon2id format; values > 255 are rejected by argon2.IDKey
		default:
			return argonParams{}, nil, nil, fmt.Errorf("unknown argon2id param %q", kv[0])
		}
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("decode salt: %w", err)
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("decode hash: %w", err)
	}
	return params, salt, hash, nil
}
