package main

import (
	"fmt"
	"os"
	"strconv"
)

// Sieve implements the Sieve of Eratosthenes algorithm to find all prime numbers up to n
func Sieve(n int) []int {
	if n < 2 {
		return []int{}
	}

	// Create a boolean array "isPrime[0..n]" and initialize all entries as true
	isPrime := make([]bool, n+1)
	for i := range isPrime {
		isPrime[i] = true
	}
	isPrime[0] = false
	isPrime[1] = false

	// Start with the smallest prime number, 2
	for p := 2; p*p <= n; p++ {
		// If isPrime[p] is not changed, then it is a prime
		if isPrime[p] {
			// Mark all multiples of p as not prime
			for i := p * p; i <= n; i += p {
				isPrime[i] = false
			}
		}
	}

	// Collect all numbers that are still marked as prime
	primes := []int{}
	for i := 2; i <= n; i++ {
		if isPrime[i] {
			primes = append(primes, i)
		}
	}

	return primes
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: sieve <n>")
		fmt.Println("Finds all prime numbers up to n using the Sieve of Eratosthenes")
		os.Exit(1)
	}

	n, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid number: %v\n", err)
		os.Exit(1)
	}

	if n < 0 {
		fmt.Fprintf(os.Stderr, "Error: n must be non-negative\n")
		os.Exit(1)
	}

	primes := Sieve(n)

	fmt.Printf("Prime numbers up to %d:\n", n)
	for i, prime := range primes {
		if i > 0 && i%10 == 0 {
			fmt.Println()
		}
		fmt.Printf("%d ", prime)
	}
	fmt.Println()
	fmt.Printf("\nTotal: %d primes\n", len(primes))
}
