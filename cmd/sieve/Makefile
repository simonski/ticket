CC = gcc
CFLAGS = -Wall -Wextra -O2 -std=c11
NIM = nim
NIMFLAGS = c -d:release
RUSTC = rustc
RUSTFLAGS = --edition 2021 -C opt-level=3
ZIG = zig
ZIGFLAGS = build-exe -O ReleaseFast
TARGET_C = sieve
TARGET_NIM = sieve-nim
TARGET_RUST = sieve-rust
TARGET_ZIG = sieve-zig
SRC_C = sieve.c
SRC_NIM = sieve.nim
SRC_RUST = sieve.rs
SRC_ZIG = sieve.zig

all: $(TARGET_C) $(TARGET_NIM) $(TARGET_RUST) $(TARGET_ZIG)

$(TARGET_C): $(SRC_C)
	$(CC) $(CFLAGS) -o $(TARGET_C) $(SRC_C)

$(TARGET_NIM): $(SRC_NIM)
	$(NIM) $(NIMFLAGS) -o:$(TARGET_NIM) $(SRC_NIM)

$(TARGET_RUST): $(SRC_RUST)
	$(RUSTC) $(RUSTFLAGS) -o $(TARGET_RUST) $(SRC_RUST)

$(TARGET_ZIG): $(SRC_ZIG)
	$(ZIG) $(ZIGFLAGS) -femit-bin=$(TARGET_ZIG) $(SRC_ZIG)

clean:
	rm -f $(TARGET_C) $(TARGET_NIM) $(TARGET_RUST) $(TARGET_ZIG)
	rm -rf nimcache

test: $(TARGET_C) $(TARGET_NIM) $(TARGET_RUST) $(TARGET_ZIG)
	@echo "Testing C implementation with n=30..."
	@./$(TARGET_C) 30
	@echo ""
	@echo "Testing Nim implementation with n=30..."
	@./$(TARGET_NIM) 30
	@echo ""
	@echo "Testing Rust implementation with n=30..."
	@./$(TARGET_RUST) 30
	@echo ""
	@echo "Testing Zig implementation with n=30..."
	@./$(TARGET_ZIG) 30

.PHONY: all clean test
