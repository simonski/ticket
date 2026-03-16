import re

files = [
    'internal/store/task.go',
    'internal/store/activity.go',
    'internal/store/dependency.go',
    'internal/store/count.go',
    'internal/server/api.go',
    'libticket/local.go',
    'internal/client/client.go',
]

for path in files:
    with open(path) as f:
        text = f.read()

    text = re.sub(r'\btaskID\b', 'ticketID', text)
    text = re.sub(r'\btaskState\b', 'ticketState', text)
    text = re.sub(r'\btaskStatus\b', 'ticketStatus', text)
    text = re.sub(r'\btaskCount\b', 'ticketCount', text)
    # taskType as Go var (not string comparison sides)
    text = re.sub(r'\btaskType\b', 'ticketType', text)

    with open(path, 'w') as f:
        f.write(text)
    print('  ' + path)

print('Done variable renames.')
