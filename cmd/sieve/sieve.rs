#!/usr/bin/env -S rustc --edition 2021 -O -o sieve-rust && ./sieve-rust
//! Sieve of Eratosthenes - Prime Number Generator
//!
//! Finds all prime numbers up to a given limit using the Sieve of Eratosthenes algorithm.

use std::env;
use std::process;

/// Generate all prime numbers up to n using the Sieve of Eratosthenes.
///
/// # Arguments
/// * `n` - The upper limit (inclusive) for finding primes
///
/// # Returns
/// A vector of all prime numbers up to n
fn sieve_of_eratosthenes(n: usize) -> Vec<usize> {
    if n < 2 {
        return Vec::new();
    }

    // Create a boolean vector "is_prime" and initialize all entries as true
    let mut is_prime = vec![true; n + 1];
    is_prime[0] = false;
    is_prime[1] = false;

    // Start with the smallest prime number, 2
    let mut p = 2;
    while p * p <= n {
        // If is_prime[p] is not changed, then it is a prime
        if is_prime[p] {
            // Mark all multiples of p as not prime
            let mut i = p * p;
            while i <= n {
                is_prime[i] = false;
                i += p;
            }
        }
        p += 1;
    }

    // Collect all numbers that are still marked as prime
    (2..=n).filter(|&i| is_prime[i]).collect()
}

fn main() {
    let args: Vec<String> = env::args().collect();

    if args.len() < 2 {
        eprintln!("Usage: sieve-rust <n>");
        eprintln!("Finds all prime numbers up to n using the Sieve of Eratosthenes");
        process::exit(1);
    }

    let n = match args[1].parse::<usize>() {
        Ok(num) => num,
        Err(_) => {
            eprintln!("Error: invalid number: {}", args[1]);
            process::exit(1);
        }
    };

    let primes = sieve_of_eratosthenes(n);

    println!("Prime numbers up to {}:", n);

    // Print primes with line breaks every 10 numbers
    for (i, prime) in primes.iter().enumerate() {
        if i > 0 && i % 10 == 0 {
            println!();
        }
        print!("{} ", prime);
    }

    if !primes.is_empty() {
        println!();
    }

    println!("\nTotal: {} primes", primes.len());
}
