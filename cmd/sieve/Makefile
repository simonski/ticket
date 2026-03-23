CC = gcc
CFLAGS = -Wall -Wextra -O2 -std=c11
NIM = nim
NIMFLAGS = c -d:release
TARGET_C = sieve
TARGET_NIM = sieve-nim
SRC_C = sieve.c
SRC_NIM = sieve.nim

all: $(TARGET_C) $(TARGET_NIM)

$(TARGET_C): $(SRC_C)
	$(CC) $(CFLAGS) -o $(TARGET_C) $(SRC_C)

$(TARGET_NIM): $(SRC_NIM)
	$(NIM) $(NIMFLAGS) -o:$(TARGET_NIM) $(SRC_NIM)

clean:
	rm -f $(TARGET_C) $(TARGET_NIM)
	rm -rf nimcache

test: $(TARGET_C) $(TARGET_NIM)
	@echo "Testing C implementation with n=30..."
	@./$(TARGET_C) 30
	@echo ""
	@echo "Testing Nim implementation with n=30..."
	@./$(TARGET_NIM) 30

.PHONY: all clean test
