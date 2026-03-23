#include <stdio.h>
#include <stdlib.h>
#include <stdbool.h>
#include <string.h>

void sieve_of_eratosthenes(int n) {
    if (n < 2) {
        printf("Prime numbers up to %d:\n", n);
        printf("\nTotal: 0 primes\n");
        return;
    }

    // Create a boolean array "is_prime[0..n]" and initialize all entries as true
    bool *is_prime = (bool *)malloc((n + 1) * sizeof(bool));
    if (is_prime == NULL) {
        fprintf(stderr, "Error: memory allocation failed\n");
        exit(1);
    }

    memset(is_prime, true, (n + 1) * sizeof(bool));
    is_prime[0] = false;
    is_prime[1] = false;

    // Start with the smallest prime number, 2
    for (int p = 2; p * p <= n; p++) {
        // If is_prime[p] is not changed, then it is a prime
        if (is_prime[p]) {
            // Mark all multiples of p as not prime
            for (int i = p * p; i <= n; i += p) {
                is_prime[i] = false;
            }
        }
    }

    // Print all numbers that are still marked as prime
    printf("Prime numbers up to %d:\n", n);
    int count = 0;
    for (int i = 2; i <= n; i++) {
        if (is_prime[i]) {
            if (count > 0 && count % 10 == 0) {
                printf("\n");
            }
            printf("%d ", i);
            count++;
        }
    }
    printf("\n\nTotal: %d primes\n", count);

    free(is_prime);
}

int main(int argc, char *argv[]) {
    if (argc < 2) {
        printf("Usage: sieve <n>\n");
        printf("Finds all prime numbers up to n using the Sieve of Eratosthenes\n");
        return 1;
    }

    char *endptr;
    long n = strtol(argv[1], &endptr, 10);

    if (*endptr != '\0' || endptr == argv[1]) {
        fprintf(stderr, "Error: invalid number: %s\n", argv[1]);
        return 1;
    }

    if (n < 0) {
        fprintf(stderr, "Error: n must be non-negative\n");
        return 1;
    }

    if (n > 2147483647) {
        fprintf(stderr, "Error: n is too large\n");
        return 1;
    }

    sieve_of_eratosthenes((int)n);

    return 0;
}
