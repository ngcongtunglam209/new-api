# CLIProxyAPI backend

CLIProxyAPI pools OAuth subscription accounts (Claude Code, Codex, Gemini/Antigravity, Grok, Kimi)
and exposes them as OpenAI-, Anthropic-, and Gemini-compatible HTTP APIs on port 8317.
new-api talks to it through a normal relay channel; no adaptor code is involved.

Upstream: https://github.com/router-for-me/CLIProxyAPI — docs: https://help.router-for.me/

## 1. Configure

```bash
cp cliproxy/config.example.yaml cliproxy/config.yaml
# edit api-keys: replace CHANGE-ME-new-api-channel-key with a long random string
```

`cliproxy/config.yaml` and `cliproxy/auth/` are gitignored — they hold the channel key and the
OAuth refresh tokens.

## 2. Log in the subscription accounts

Each login runs the same image with a login flag and writes a credential file into `cliproxy/auth/`.
Repeat per account; CLIProxyAPI round-robins across every credential file it finds.

```bash
# Claude Code (OAuth callback on 54545)
docker run --rm -it -p 54545:54545 \
  -v "$PWD/cliproxy/config.yaml:/CLIProxyAPI/config.yaml" \
  -v "$PWD/cliproxy/auth:/root/.cli-proxy-api" \
  eceasy/cli-proxy-api:latest --claude-login --no-browser

# Codex / ChatGPT (OAuth callback on 1455)
docker run --rm -it -p 1455:1455 \
  -v "$PWD/cliproxy/config.yaml:/CLIProxyAPI/config.yaml" \
  -v "$PWD/cliproxy/auth:/root/.cli-proxy-api" \
  eceasy/cli-proxy-api:latest --codex-login --no-browser
```

`--no-browser` prints the authorization URL; open it on your own machine. The callback returns to
`http://localhost:<port>/...`, so either run the login on the machine with the browser, or SSH
forward that port to the server (`ssh -L 54545:localhost:54545 root@<vps>`).

Other flags from the same image: `--gemini-login`(`--login`), `--antigravity-login`,
`--kimi-login`, `--xai-login`, `--codex-device-login` (device code, no callback port needed),
`--vertex-import <key.json>`.

## 3. Start

```bash
docker compose up -d cliproxyapi
curl -H "Authorization: Bearer <api-key>" http://127.0.0.1:8317/v1/models
```

## 4. Wire the new-api channel

Channels → add:

- Type: **Sub2API** (an OpenAI/Anthropic/Gemini passthrough channel; it forwards the request
  format untouched and sends `Authorization`, `x-api-key`, and `x-goog-api-key`)
- Base URL: `http://cliproxyapi:8317`
- Key: the value from `api-keys`
- Models: use "fetch models" — it reads `/v1/models` from CLIProxyAPI

Test the channel, then disable the old Sub2API-backed channel and stop that stack.
