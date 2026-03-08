import re

files = [
    'internal/store/ticket_test.go',
    'internal/server/api_test.go',
    'internal/client/client_local_test.go',
]

def rename_identifiers(text):
    result = []
    i = 0
    in_string = False
    in_backtick = False
    while i < len(text):
        ch = text[i]
        if ch == '\\' and in_string:
            result.append(text[i:i+2])
            i += 2
            continue
        if ch == '"' and not in_backtick:
            in_string = not in_string
            result.append(ch)
            i += 1
            continue
        if ch == '`' and not in_string:
            in_backtick = not in_backtick
            result.append(ch)
            i += 1
            continue
        if not in_string and not in_backtick:
            # Check word boundary before
            prev_ok = (i == 0 or (not text[i-1].isalnum() and text[i-1] != '_'))
            if prev_ok:
                # Check 'tasks' (longer first)
                if text[i:i+5] == 'tasks':
                    after = i + 5
                    if after >= len(text) or (not text[after].isalnum() and text[after] != '_'):
                        result.append('tickets')
                        i += 5
                        continue
                # Check 'task'
                if text[i:i+4] == 'task':
                    after = i + 4
                    if after >= len(text) or (not text[after].isalnum() and text[after] != '_'):
                        result.append('ticket')
                        i += 4
                        continue
        result.append(ch)
        i += 1
    return ''.join(result)

for path in files:
    try:
        with open(path) as f:
            text = f.read()
        new_text = rename_identifiers(text)
        if new_text != text:
            with open(path, 'w') as f:
                f.write(new_text)
            print('  updated: ' + path)
        else:
            print('  unchanged: ' + path)
    except FileNotFoundError:
        print('  not found: ' + path)

print('Done.')
