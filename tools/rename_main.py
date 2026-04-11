import re

# More comprehensive fix for main.go: rename task/tasks as Go identifiers
# Strategy: replace word-boundary 'task' and 'tasks' ONLY when they're identifiers
# (not inside double-quoted strings)

path = 'cmd/tk/main.go'

with open(path) as f:
    text = f.read()

# Split on double-quoted strings to avoid replacing inside them
# We'll use tokenization approach: process tokens outside strings

result = []
# Simple tokenizer: split at " boundaries
i = 0
in_string = False
while i < len(text):
    ch = text[i]
    if ch == '\\' and in_string:
        result.append(text[i:i+2])
        i += 2
        continue
    if ch == '"':
        in_string = not in_string
        result.append(ch)
        i += 1
        continue
    if not in_string:
        # Replace task/tasks as identifiers
        # Check for 'tasks' first (longer match)
        if text[i:].startswith('tasks') and (i == 0 or not text[i-1].isalnum() and text[i-1] != '_'):
            after = i + 5
            if after >= len(text) or (not text[after].isalnum() and text[after] != '_'):
                result.append('tickets')
                i += 5
                continue
        # Check for 'task' as identifier
        if text[i:].startswith('task') and (i == 0 or not text[i-1].isalnum() and text[i-1] != '_'):
            after = i + 4
            if after >= len(text) or (not text[after].isalnum() and text[after] != '_'):
                result.append('ticket')
                i += 4
                continue
    result.append(ch)
    i += 1

new_text = ''.join(result)

with open(path, 'w') as f:
    f.write(new_text)

print(f'Done: {path}')
# Count remaining task references (excluding string literals)
remaining = [(i+1, l) for i, l in enumerate(new_text.split('\n')) if re.search(r'\btask\b', l) and '"task"' not in l and '"tasks"' not in l]
print(f'Remaining task refs (non-string): {len(remaining)}')
for lineno, line in remaining[:20]:
    print(f'  L{lineno}: {line.strip()}')
