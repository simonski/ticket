#!/usr/bin/env perl
#
# Sieve of Eratosthenes - Prime Number Generator
#
# Finds all prime numbers up to a given limit using the Sieve of Eratosthenes algorithm.

use strict;
use warnings;
use v5.10;

# Generate all prime numbers up to n using the Sieve of Eratosthenes.
#
# Args:
#     n - The upper limit (inclusive) for finding primes
#
# Returns:
#     An array reference of all prime numbers up to n
sub sieve_of_eratosthenes {
    my ($n) = @_;

    return [] if $n < 2;

    # Create a boolean array "is_prime" and initialize all entries as true
    my @is_prime = (1) x ($n + 1);
    $is_prime[0] = 0;
    $is_prime[1] = 0;

    # Start with the smallest prime number, 2
    my $p = 2;
    while ($p * $p <= $n) {
        # If is_prime[p] is not changed, then it is a prime
        if ($is_prime[$p]) {
            # Mark all multiples of p as not prime
            for (my $i = $p * $p; $i <= $n; $i += $p) {
                $is_prime[$i] = 0;
            }
        }
        $p++;
    }

    # Collect all numbers that are still marked as prime
    my @primes;
    for my $i (2..$n) {
        push @primes, $i if $is_prime[$i];
    }

    return \@primes;
}

sub main {
    if (@ARGV < 1) {
        say STDERR "Usage: sieve.pl <n>";
        say STDERR "Finds all prime numbers up to n using the Sieve of Eratosthenes";
        exit 1;
    }

    my $n = $ARGV[0];

    # Validate input is a number
    unless ($n =~ /^\d+$/) {
        say STDERR "Error: invalid number: $n";
        exit 1;
    }

    my $primes = sieve_of_eratosthenes($n);

    say "Prime numbers up to $n:";

    # Print primes with line breaks every 10 numbers
    for my $i (0..$#$primes) {
        print "\n" if $i > 0 && $i % 10 == 0;
        print "$primes->[$i] ";
    }

    print "\n" if @$primes;

    say "\nTotal: " . scalar(@$primes) . " primes";
}

main();
