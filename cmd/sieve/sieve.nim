import std/[os, strutils]

proc sieveOfEratosthenes(n: int) =
  if n < 2:
    echo "Prime numbers up to ", n, ":"
    echo ""
    echo "Total: 0 primes"
    return

  # Create a boolean sequence and initialize all entries as true
  var isPrime = newSeq[bool](n + 1)
  for i in 0..n:
    isPrime[i] = true

  isPrime[0] = false
  isPrime[1] = false

  # Start with the smallest prime number, 2
  var p = 2
  while p * p <= n:
    # If isPrime[p] is not changed, then it is a prime
    if isPrime[p]:
      # Mark all multiples of p as not prime
      var i = p * p
      while i <= n:
        isPrime[i] = false
        i += p
    inc p

  # Print all numbers that are still marked as prime
  echo "Prime numbers up to ", n, ":"
  var count = 0
  for i in 2..n:
    if isPrime[i]:
      if count > 0 and count mod 10 == 0:
        echo ""
      stdout.write i, " "
      inc count

  echo "\n"
  echo "Total: ", count, " primes"

when isMainModule:
  if paramCount() < 1:
    echo "Usage: sieve <n>"
    echo "Finds all prime numbers up to n using the Sieve of Eratosthenes"
    quit(1)

  let arg = paramStr(1)

  try:
    let n = parseInt(arg)

    if n < 0:
      stderr.writeLine "Error: n must be non-negative"
      quit(1)

    sieveOfEratosthenes(n)
  except ValueError:
    stderr.writeLine "Error: invalid number: ", arg
    quit(1)
