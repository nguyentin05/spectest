# Installation Guide

SpecTest is distributed as a single, self-contained binary for Linux, macOS, and Windows with zero external runtime dependencies.

---

## Prebuilt Binaries (GitHub Releases)

Download pre-compiled binaries from the official [SpecTest GitHub Releases](https://github.com/nguyentin05/spectest/releases) page.

=== "Linux (x86_64 / amd64)"
    ```bash
    curl -sSL https://github.com/nguyentin05/spectest/releases/latest/download/spectest_linux_amd64.tar.gz | tar -xz
    sudo mv spectest /usr/local/bin/
    spectest --version
    ```

=== "Linux (ARM64 / aarch64)"
    ```bash
    curl -sSL https://github.com/nguyentin05/spectest/releases/latest/download/spectest_linux_arm64.tar.gz | tar -xz
    sudo mv spectest /usr/local/bin/
    spectest --version
    ```

=== "macOS (Apple Silicon - M1/M2/M3)"
    ```bash
    curl -sSL https://github.com/nguyentin05/spectest/releases/latest/download/spectest_darwin_arm64.tar.gz | tar -xz
    sudo mv spectest /usr/local/bin/
    spectest --version
    ```

=== "macOS (Intel - x86_64)"
    ```bash
    curl -sSL https://github.com/nguyentin05/spectest/releases/latest/download/spectest_darwin_amd64.tar.gz | tar -xz
    sudo mv spectest /usr/local/bin/
    spectest --version
    ```

=== "Windows"
    Download `spectest_windows_amd64.zip` from [Releases](https://github.com/nguyentin05/spectest/releases), extract `spectest.exe`, and add its folder to your system `PATH`.

---

## Go Toolchain

If you have Go (version 1.22 or newer) installed:

```bash
go install github.com/nguyentin05/spectest/cmd/spectest@latest
```

Ensure `$GOPATH/bin` or `$HOME/go/bin` is in your environment's `PATH`:

```bash
spectest --version
```

---

## Building from Source

To build directly from Git:

```bash
git clone https://github.com/nguyentin05/spectest.git
cd spectest

# Compile binary to ./bin/spectest
make build

# Run tests
make test
```

---

## Shell Autocompletion

Generate autocompletion scripts for your shell:

=== "Bash"
    ```bash
    spectest completion bash > /etc/bash_completion.d/spectest
    ```

=== "Zsh"
    ```bash
    spectest completion zsh > "${fpath[1]}/_spectest"
    ```

=== "Fish"
    ```bash
    spectest completion fish > ~/.config/fish/completions/spectest.fish
    ```

=== "PowerShell"
    ```powershell
    spectest completion powershell | Out-String | Invoke-Expression
    ```
