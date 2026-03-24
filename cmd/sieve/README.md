# Sieve - Prime Number Generator

Implementations of the Sieve of Eratosthenes algorithm to find all prime numbers up to a given limit.

Available in C, Go, Nim, Node.js, Perl, Python, Rust, and Zig.

## C Implementation

```bash
# Build the program
make

# Run with a limit
./sieve 100

# Clean build artifacts
make clean
```

## Go Implementation

```bash
# Build the program
go build -o bin/sieve ./cmd/sieve/

# Run with a limit
./bin/sieve 100
```

## Nim Implementation

```bash
# Build the program
nim c -d:release -o:sieve-nim sieve.nim

# Run with a limit
./sieve-nim 100
```

## Node.js Implementation

```bash
# Run directly (no build needed)
node sieve.js 100

# Or make it executable and run
chmod +x sieve.js
./sieve.js 100
```

## Perl Implementation

```bash
# Run directly (no build needed)
perl sieve.pl 100

# Or make it executable and run
chmod +x sieve.pl
./sieve.pl 100
```

## Python Implementation

```bash
# Run directly (no build needed)
python3 sieve.py 100

# Or make it executable and run
chmod +x sieve.py
./sieve.py 100
```

## Rust Implementation

```bash
# Build the program
rustc --edition 2021 -C opt-level=3 -o sieve-rust sieve.rs

# Or use make
make sieve-rust

# Run with a limit
./sieve-rust 100
```

## Zig Implementation

```bash
# Build the program
zig build-exe -O ReleaseFast -femit-bin=sieve-zig sieve.zig

# Or use make
make sieve-zig

# Run with a limit
./sieve-zig 100
```

## Example

```bash
$ ./sieve 30
Prime numbers up to 30:
2 3 5 7 11 13 17 19 23 29

Total: 10 primes
```

## Algorithm

The Sieve of Eratosthenes is an ancient algorithm for finding all prime numbers up to a given limit. It works by:

1. Creating a list of consecutive integers from 2 to n
2. Starting with the smallest prime (2), marking all its multiples as composite
3. Moving to the next unmarked number and repeating
4. Continuing until all numbers have been processed

Time complexity: O(n log log n)
Space complexity: O(n)

## Testing

```bash
go test ./cmd/sieve/
```
