#!/usr/bin/env node
/**
 * Sieve of Eratosthenes - Prime Number Generator
 *
 * Finds all prime numbers up to a given limit using the Sieve of Eratosthenes algorithm.
 */

/**
 * Generate all prime numbers up to n using the Sieve of Eratosthenes.
 *
 * @param {number} n - The upper limit (inclusive) for finding primes
 * @returns {number[]} An array of all prime numbers up to n
 */
function sieveOfEratosthenes(n) {
    if (n < 2) {
        return [];
    }

    // Create a boolean array "isPrime" and initialize all entries as true
    const isPrime = new Array(n + 1).fill(true);
    isPrime[0] = false;
    isPrime[1] = false;

    // Start with the smallest prime number, 2
    let p = 2;
    while (p * p <= n) {
        // If isPrime[p] is not changed, then it is a prime
        if (isPrime[p]) {
            // Mark all multiples of p as not prime
            for (let i = p * p; i <= n; i += p) {
                isPrime[i] = false;
            }
        }
        p++;
    }

    // Collect all numbers that are still marked as prime
    const primes = [];
    for (let i = 2; i <= n; i++) {
        if (isPrime[i]) {
            primes.push(i);
        }
    }

    return primes;
}

/**
 * Main entry point for the program.
 */
function main() {
    if (process.argv.length < 3) {
        console.log("Usage: sieve.js <n>");
        console.log("Finds all prime numbers up to n using the Sieve of Eratosthenes");
        process.exit(1);
    }

    const n = parseInt(process.argv[2], 10);

    if (isNaN(n)) {
        console.error(`Error: invalid number: ${process.argv[2]}`);
        process.exit(1);
    }

    if (n < 0) {
        console.error("Error: n must be non-negative");
        process.exit(1);
    }

    const primes = sieveOfEratosthenes(n);

    console.log(`Prime numbers up to ${n}:`);

    // Print primes with line breaks every 10 numbers
    let output = '';
    for (let i = 0; i < primes.length; i++) {
        if (i > 0 && i % 10 === 0) {
            output += '\n';
        }
        output += primes[i] + ' ';
    }

    if (primes.length > 0) {
        console.log(output);
    }

    console.log(`\nTotal: ${primes.length} primes`);
}

main();
