import re

# Update user-facing strings where "task" means a ticket (not the ticket type value)
# Don't change: case "task": / type == "task" / == "task" / "task," in type lists

files = [
    'cmd/tk/main.go',
    'internal/store/task.go',
    'internal/server/api.go',
    'internal/store/dependency.go',
    'internal/store/activity.go',
]

def update_strings(text):
    # Error/help messages: "task is already assigned", "task is not assigned", etc.
    # Match only in string literals (Go double-quoted strings)
    def replace_in_string(m):
        s = m.group(0)
        # Don't replace if it's a type comparison value (surrounded by quotes alone)
        if re.match(r'^"task"$', s) or re.match(r'^"tasks"$', s):
            return s
        # Replace task-> ticket inside the string content
        inner = s[1:-1]  # strip quotes
        inner = re.sub(r'\btask\b', 'ticket', inner)
        inner = re.sub(r'\btasks\b', 'tickets', inner)
        return '"' + inner + '"'

    # Apply to double-quoted strings only
    text = re.sub(r'"[^"\n]*"', replace_in_string, text)
    return text

for path in files:
    try:
        with open(path) as f:
            text = f.read()
        new_text = update_strings(text)
        if new_text != text:
            with open(path, 'w') as f:
                f.write(new_text)
            print('  updated: ' + path)
        else:
            print('  unchanged: ' + path)
    except FileNotFoundError:
        print('  not found: ' + path)

print('Done.')
