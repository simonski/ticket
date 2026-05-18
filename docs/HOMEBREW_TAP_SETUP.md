# Setting up simonski/homebrew-tap

## One-time setup

Create a **public** GitHub repository named exactly `homebrew-tap` under your
account (`github.com/simonski/homebrew-tap`).  Homebrew derives the tap name
from the repo name: `simonski/homebrew-tap` → `brew tap simonski/tap`.

### Repo structure

```
homebrew-tap/
└── Formula/
    └── ticket.rb      ← copied here after each release
```

### Bootstrap

```bash
mkdir homebrew-tap && cd homebrew-tap
git init
mkdir Formula
echo "# simonski/tap" > README.md
git add . && git commit -m "init"
gh repo create simonski/homebrew-tap --public --source=. --push
```

## Each release

1. In the ticket repo:
   ```bash
   make release          # builds all platforms, creates the GitHub release, and updates the tap
   ```

2. To push the formula to the tap separately:
   ```bash
   make homebrew
   ```

## Users install with

```bash
brew tap simonski/tap
brew install ticket
```

Or in one line (no prior tap needed):

```bash
brew install simonski/tap/ticket
```

Both `ticket` and `tk` commands are installed.
