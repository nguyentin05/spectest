# Credentials & Provider Configuration

SpecTest supports both commercial cloud LLM APIs (Google Gemini, OpenAI) and local, self-hosted offline models (Ollama, vLLM, LocalAI).

---

## Storage & File Permissions

Provider settings and API keys are stored in:

```text
~/.spectest/credentials.yaml
```

- **File Permissions**: Created strictly with mode `0600` (`-rw-------`), allowing only the current user to read and write.
- **Directory Permissions**: Parent directory `~/.spectest/` is created with mode `0700` (`drwx------`).
- **Terminal Masking**: Input prompts disable terminal echo to prevent keys from appearing in command-line histories or shoulder-surfing.
- **Display Masking**: The `spectest config list` command automatically masks secret keys: `AIzaSy************************3X81`.

---

## Supported Providers

### 1. Google Gemini (Cloud API)
Optimized for high-throughput scenario synthesis and low-cost AI defect triage.

```bash
spectest config init --provider gemini --model gemini-2.5-flash
spectest config set api-key
```

- **Default Endpoint**: `https://generativelanguage.googleapis.com/v1beta/openai/`
- **Recommended Models**: `gemini-2.5-flash`, `gemini-1.5-pro`

### 2. OpenAI & OpenAI-Compatible Gateways
Works with OpenAI as well as gateways implementing OpenAI-compatible REST endpoints (LiteLLM, Groq, OpenRouter).

```bash
spectest config init \
  --provider openai \
  --endpoint https://api.openai.com/v1 \
  --model gpt-4o-mini

spectest config set api-key
```

### 3. Ollama (Local / Air-Gapped / Offline)
Enables complete offline operation. No API contracts or test data ever leave your machine:

1. Install and start [Ollama](https://ollama.ai):
   ```bash
   ollama pull qwen2.5-coder:7b
   ollama serve
   ```
2. Configure SpecTest to point to Ollama's OpenAI-compatible endpoint:
   ```bash
   spectest config init \
     --provider ollama \
     --endpoint http://localhost:11434/v1 \
     --model qwen2.5-coder:7b
   ```
   *(No API key required).*

---

## Environment Variable Overrides

In automated CI/CD runners (GitHub Actions, GitLab CI, Jenkins), credentials can be injected via environment variables without creating `~/.spectest/credentials.yaml`:

```bash
# General API Key override
export SPECTEST_API_KEY="AIzaSyYourSecretKey"

# Specific Provider Fallbacks
export GEMINI_API_KEY="your-gemini-key"
export OPENAI_API_KEY="your-openai-key"

# Custom config directory (e.g. for containerized runners)
export SPECTEST_CONFIG_DIR="/tmp/.spectest"
```
