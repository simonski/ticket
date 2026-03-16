import re

# Rename task/tasks as Go local variables (not string literals)
# Strategy: replace in variable declaration patterns, loop variables, and struct access

files = [
    'cmd/ticket/main.go',
    'internal/store/task.go',
    'internal/store/activity.go',
    'internal/store/dependency.go',
    'libticket/local.go',
    'internal/client/client.go',
    'libtickettest/contract.go',
    'libticket/local_test.go',
    'internal/client/client_local_test.go',
    'internal/client/client_http_test.go',
    'internal/store/task_test.go',
    'internal/store/store_test.go',
    'internal/store/activity_test.go',
    'internal/store/count_test.go',
    'internal/store/dependency_spec_test.go',
    'internal/server/api_test.go',
    'libtickethttp/http_test.go',
]

def rename_vars(text):
    # Rename 'tasks' (slice variable) -> 'tickets' in Go code patterns
    # Careful: don't rename inside string literals
    # We'll do targeted patterns for declaration and assignment

    # for _, task := range -> for _, ticket := range
    text = re.sub(r'\bfor\s+(_|\w+),\s+task\s+:=\s+range\b', lambda m: m.group(0).replace(' task ', ' ticket '), text)
    # for _, tasks := range  (unlikely but)
    text = re.sub(r'\bfor\s+(_|\w+),\s+tasks\s+:=\s+range\b', lambda m: m.group(0).replace(' tasks ', ' tickets '), text)

    # var task store.Ticket / var task Ticket
    text = re.sub(r'\bvar task\b', 'var ticket', text)
    # var tasks []
    text = re.sub(r'\bvar tasks\b', 'var tickets', text)

    # task, err := / task := 
    text = re.sub(r'\btask, err :=', 'ticket, err :=', text)
    text = re.sub(r'\btask :=', 'ticket :=', text)
    text = re.sub(r'\btasks :=', 'tickets :=', text)
    text = re.sub(r'\btasks, err :=', 'tickets, err :=', text)

    # task = / tasks =  (assignment)
    text = re.sub(r'\btask = ', 'ticket = ', text)
    text = re.sub(r'\btasks = ', 'tickets = ', text)

    # task. (struct field access)
    text = re.sub(r'\btask\.', 'ticket.', text)
    # tasks. (method call)
    text = re.sub(r'\btasks\.', 'tickets.', text)

    # return task / return tasks
    text = re.sub(r'\breturn task\b', 'return ticket', text)
    text = re.sub(r'\breturn tasks\b', 'return tickets', text)

    # append(tasks, / append(task,
    text = re.sub(r'\bappend\(tasks,', 'append(tickets,', text)

    # function params named task or tasks
    text = re.sub(r'\(task store\.', '(ticket store.', text)
    text = re.sub(r'\(tasks \[\]', '(tickets []', text)
    text = re.sub(r',\s*task store\.', ', ticket store.', text)
    text = re.sub(r',\s*tasks \[\]', ', tickets []', text)

    # orphans = append(orphans, task) -> ticket
    # handled by task. above

    # specific: &task / &tasks
    text = re.sub(r'&task\b', '&ticket', text)

    # response.Ticket = &task -> &ticket (already handled)
    # draft.ticket = created (already ticket in html, skip)

    # for _, task range in test files
    text = re.sub(r'\bfor _, task\b', 'for _, ticket', text)

    # task, status, err
    text = re.sub(r'\btask, status, err\b', 'ticket, status, err', text)
    text = re.sub(r'\btask, "ASSIGNED"\b', 'ticket, "ASSIGNED"', text)
    text = re.sub(r'\btask, "AVAILABLE"\b', 'ticket, "AVAILABLE"', text)

    return text

for path in files:
    try:
        with open(path) as f:
            text = f.read()
        new_text = rename_vars(text)
        if new_text != text:
            with open(path, 'w') as f:
                f.write(new_text)
            print('  updated: ' + path)
        else:
            print('  unchanged: ' + path)
    except FileNotFoundError:
        print('  not found: ' + path)

print('Done.')
