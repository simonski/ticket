const std = @import("std");

/// Generate all prime numbers up to n using the Sieve of Eratosthenes.
fn sieveOfEratosthenes(allocator: std.mem.Allocator, n: usize) !std.array_list.Managed(usize) {
    var primes = std.array_list.Managed(usize).init(allocator);
    errdefer primes.deinit();

    if (n < 2) {
        return primes;
    }

    // Create a boolean array "is_prime" and initialize all entries as true
    const is_prime = try allocator.alloc(bool, n + 1);
    defer allocator.free(is_prime);

    @memset(is_prime, true);
    is_prime[0] = false;
    is_prime[1] = false;

    // Start with the smallest prime number, 2
    var p: usize = 2;
    while (p * p <= n) : (p += 1) {
        // If is_prime[p] is not changed, then it is a prime
        if (is_prime[p]) {
            // Mark all multiples of p as not prime
            var i: usize = p * p;
            while (i <= n) : (i += p) {
                is_prime[i] = false;
            }
        }
    }

    // Collect all numbers that are still marked as prime
    var i: usize = 2;
    while (i <= n) : (i += 1) {
        if (is_prime[i]) {
            try primes.append(i);
        }
    }

    return primes;
}

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    const args = try std.process.argsAlloc(allocator);
    defer std.process.argsFree(allocator, args);

    const stderr = std.fs.File.stderr();
    const stdout = std.fs.File.stdout();

    if (args.len < 2) {
        _ = try stderr.write("Usage: sieve-zig <n>\n");
        _ = try stderr.write("Finds all prime numbers up to n using the Sieve of Eratosthenes\n");
        std.process.exit(1);
    }

    const n = std.fmt.parseInt(usize, args[1], 10) catch {
        var buf: [256]u8 = undefined;
        const msg = try std.fmt.bufPrint(&buf, "Error: invalid number: {s}\n", .{args[1]});
        _ = try stderr.write(msg);
        std.process.exit(1);
    };

    const primes = try sieveOfEratosthenes(allocator, n);
    defer primes.deinit();

    // Print header
    var buf: [256]u8 = undefined;
    const header = try std.fmt.bufPrint(&buf, "Prime numbers up to {d}:\n", .{n});
    _ = try stdout.write(header);

    // Print primes with line breaks every 10 numbers
    for (primes.items, 0..) |prime, i| {
        if (i > 0 and i % 10 == 0) {
            _ = try stdout.write("\n");
        }
        const prime_str = try std.fmt.bufPrint(&buf, "{d} ", .{prime});
        _ = try stdout.write(prime_str);
    }

    if (primes.items.len > 0) {
        _ = try stdout.write("\n");
    }

    const footer = try std.fmt.bufPrint(&buf, "\nTotal: {d} primes\n", .{primes.items.len});
    _ = try stdout.write(footer);
}
