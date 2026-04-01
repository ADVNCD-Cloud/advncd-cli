# Publishing advncd to Homebrew

This guide explains how to make `brew install advncd` work.

There are two paths:
- **Option A** — Your own tap (`brew install advncd-cloud/tap/advncd`). Ready in ~30 minutes. No review required.
- **Option B** — Homebrew Core (`brew install advncd`). Short URL, but requires open-source license, significant user base, and passing a review. Do Option A first.

---

## Option A — Homebrew Tap (recommended first step)

### Step 1 — Create a release on GitHub

You need a tagged release with pre-built binaries for each platform.

**1a. Create a GoReleaser config** in the repo root:

```yaml
# .goreleaser.yaml
project_name: advncd
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - main: .
    binary: advncd
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64

archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip

checksum:
  name_template: "checksums.txt"

release:
  github:
    owner: ADVNCD-Cloud
    name: advncd-cli
```

**1b. Install GoReleaser:**

```bash
brew install goreleaser
```

**1c. Tag and release:**

```bash
git tag v0.2.2
git push origin v0.2.2
goreleaser release --clean
```

This produces GitHub release assets like:
```
advncd_darwin_amd64.tar.gz
advncd_darwin_arm64.tar.gz
advncd_linux_amd64.tar.gz
advncd_linux_arm64.tar.gz
checksums.txt
```

---

### Step 2 — Create a tap repository

Create a new GitHub repo named **`homebrew-tap`** under the `ADVNCD-Cloud` org:

```
https://github.com/ADVNCD-Cloud/homebrew-tap
```

The repo must be named exactly `homebrew-tap` — Homebrew uses this naming convention.

---

### Step 3 — Write the formula

Get the SHA256 of each archive from `checksums.txt` in the release, then create:

**`Formula/advncd.rb`** in the `homebrew-tap` repo:

```ruby
class Advncd < Formula
  desc "Local-first CLI for deploying any stack to Google Cloud Run"
  homepage "https://advncd.dev"
  version "0.2.2"
  license "Noncommercial"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/ADVNCD-Cloud/advncd-cli/releases/download/v0.2.2/advncd_darwin_arm64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_FROM_checksums.txt"
    else
      url "https://github.com/ADVNCD-Cloud/advncd-cli/releases/download/v0.2.2/advncd_darwin_amd64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_FROM_checksums.txt"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/ADVNCD-Cloud/advncd-cli/releases/download/v0.2.2/advncd_linux_arm64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_FROM_checksums.txt"
    else
      url "https://github.com/ADVNCD-Cloud/advncd-cli/releases/download/v0.2.2/advncd_linux_amd64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_FROM_checksums.txt"
    end
  end

  def install
    bin.install "advncd"
  end

  test do
    assert_match "advncd", shell_output("#{bin}/advncd --help")
  end
end
```

To get the SHA256 values without looking them up manually:

```bash
curl -sL https://github.com/ADVNCD-Cloud/advncd-cli/releases/download/v0.2.2/checksums.txt
```

---

### Step 4 — Test locally

```bash
brew tap ADVNCD-Cloud/tap https://github.com/ADVNCD-Cloud/homebrew-tap
brew install advncd-cloud/tap/advncd
advncd --version
```

---

### Step 5 — Users install with

```bash
brew tap ADVNCD-Cloud/tap
brew install advncd
```

Or in one line:

```bash
brew install ADVNCD-Cloud/tap/advncd
```

---

### Updating for a new release

1. Tag and run GoReleaser: `goreleaser release --clean`
2. Get new SHA256s from `checksums.txt` in the new release
3. Update `version`, `url`, and `sha256` in `Formula/advncd.rb`
4. Commit and push to `homebrew-tap`

**Automate this with GoReleaser** — add to `.goreleaser.yaml`:

```yaml
brews:
  - name: advncd
    repository:
      owner: ADVNCD-Cloud
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    homepage: "https://advncd.dev"
    description: "Local-first CLI for deploying any stack to Google Cloud Run"
    license: :cannot_represent
    test: |
      system "#{bin}/advncd", "--help"
    install: |
      bin.install "advncd"
```

Set `HOMEBREW_TAP_TOKEN` as a GitHub Actions secret (a PAT with `repo` scope on the `homebrew-tap` repo). GoReleaser will update the formula automatically on every tagged release.

---

## Option B — Homebrew Core (`brew install advncd`)

Requirements before submitting:
- Open-source license (PolyForm Noncommercial may be rejected — Core requires OSI-approved licenses)
- Stable, versioned releases on GitHub
- Notable usage (Core reviewers check this)
- Formula passes `brew audit --strict --online advncd`

If the license requirement is a blocker, Option A (tap) is the right long-term home.

**When ready:**

```bash
brew tap homebrew/core
# copy your formula to $(brew --repository homebrew/core)/Formula/a/advncd.rb
brew audit --strict --online advncd
brew install --build-from-source advncd
# open a PR at https://github.com/Homebrew/homebrew-core
```

---

## Quick reference

| Goal | Command |
|------|---------|
| Add tap | `brew tap ADVNCD-Cloud/tap` |
| Install | `brew install advncd` |
| Install (one line) | `brew install ADVNCD-Cloud/tap/advncd` |
| Upgrade | `brew upgrade advncd` |
| Uninstall | `brew uninstall advncd` |
