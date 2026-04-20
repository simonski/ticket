# atom exe.dev deployment bundle

This directory contains files copied by `make deploy`:

- `tk` - Linux server binary
- `README.md` - this file

## Quick start (binary)

```bash
chmod +x ./tk
./tk initdb -f ticket.db --force --populate 10 --password password
./tk server -f ticket.db --port 8000
```

