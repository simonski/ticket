#!/usr/bin/env python3
"""
Sieve of Eratosthenes - Prime Number Generator

Finds all prime numbers up to a given limit using the Sieve of Eratosthenes algorithm.
"""

import sys


def sieve_of_eratosthenes(n):
    """
    Generate all prime numbers up to n using the Sieve of Eratosthenes.

    Args:
        n: The upper limit (inclusive) for finding primes

    Returns:
        A list of all prime numbers up to n
    """
    if n < 2:
        return []

    # Create a boolean array "is_prime" and initialize all entries as True
    is_prime = [True] * (n + 1)
    is_prime[0] = False
    is_prime[1] = False

    # Start with the smallest prime number, 2
    p = 2
    while p * p <= n:
        # If is_prime[p] is not changed, then it is a prime
        if is_prime[p]:
            # Mark all multiples of p as not prime
            for i in range(p * p, n + 1, p):
                is_prime[i] = False
        p += 1

    # Collect all numbers that are still marked as prime
    primes = [i for i in range(2, n + 1) if is_prime[i]]
    return primes


def main():
    """Main entry point for the program."""
    if len(sys.argv) < 2:
        print("Usage: sieve.py <n>")
        print("Finds all prime numbers up to n using the Sieve of Eratosthenes")
        sys.exit(1)

    try:
        n = int(sys.argv[1])
    except ValueError:
        print(f"Error: invalid number: {sys.argv[1]}", file=sys.stderr)
        sys.exit(1)

    if n < 0:
        print("Error: n must be non-negative", file=sys.stderr)
        sys.exit(1)

    primes = sieve_of_eratosthenes(n)

    print(f"Prime numbers up to {n}:")

    # Print primes with line breaks every 10 numbers
    for i, prime in enumerate(primes):
        if i > 0 and i % 10 == 0:
            print()
        print(prime, end=" ")

    if primes:
        print()

    print(f"\nTotal: {len(primes)} primes")


if __name__ == "__main__":
    main()
