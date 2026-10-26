# Hướng dẫn Cài đặt

SpecTest được phân phối dưới dạng một file thực thi (binary) độc lập, duy nhất cho Linux, macOS và Windows, không đòi hỏi bất kỳ dependency nào khác khi chạy.

---

## File Biên dịch sẵn (GitHub Releases)

Tải file binary biên dịch sẵn từ trang chính thức [SpecTest GitHub Releases](https://github.com/nguyentin05/spectest/releases).

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
    Tải file `spectest_windows_amd64.zip` từ trang [Releases](https://github.com/nguyentin05/spectest/releases), giải nén `spectest.exe` và thêm thư mục chứa vào biến môi trường `PATH`.

---

## Cài đặt qua Go Toolchain

Nếu bạn đã cài sẵn Go (phiên bản 1.22 trở lên):

```bash
go install github.com/nguyentin05/spectest/cmd/spectest@latest
```

Đảm bảo rằng `$GOPATH/bin` hoặc `$HOME/go/bin` đã có trong biến môi trường `PATH`:

```bash
spectest --version
```

---

## Tự biên dịch từ mã nguồn

Để biên dịch trực tiếp từ Git repository:

```bash
git clone https://github.com/nguyentin05/spectest.git
cd spectest

# Biên dịch ra file ./bin/spectest
make build

# Chạy test kiểm thử
make test
```

---

## Tự động hoàn thành lệnh trong Terminal (Shell Autocompletion)

Tạo script tự động gợi ý lệnh cho shell của bạn:

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
