# llama.cpp quickstarts for the eval sweep

Ready-to-run `llama-server` commands for the two reference Gemma configs. These
carry the same context-size / `temperature` / `top_p` / `top_k` / `min_p` /
`repeat_penalty` settings the profiles expect, passed as `llama-server` flags.
The harness then talks to
`http://localhost:8080/v1`.

## Official Gemma 4 QAT (default)

Google's `gemma-4-12B-it-qat-q4_0-gguf`, with Google's own sampling preset:

```bash
llama-server -hf google/gemma-4-12B-it-qat-q4_0-gguf \
    --host 127.0.0.1 --port 8080 --ctx-size 65536 --jinja -fa on \
    --temp 1.0 --top-p 0.95 --top-k 64
```

## Stock Google Gemma-4-12B-IT (QAT Q4_0)

Google's official sampling:

```bash
llama-server -hf google/gemma-4-12B-it-qat-q4_0-gguf \
    --host 127.0.0.1 --port 8080 --ctx-size 65536 --jinja -fa on \
    --temp 1.0 --top-p 0.95 --top-k 64
```

## Notes

- `--jinja` enables the model's chat template so tool calls parse.
- `-fa on` is flash attention (exact, not lossy) — faster long-context prefill,
  smaller attention footprint, and required before the KV-cache flags below.
- `--ctx-size` must be large — tool outputs are big; 65536 matches the profiles.
- Verify the exact `-hf` quant tag on the model card (the `:Q4_K_M` suffix, etc.).
- Add `-ngl 99` to offload all layers to the GPU.
- **VRAM-constrained?** Add `-ctk q8_0 -ctv q8_0` (needs `-fa on`) to ~halve the
  KV-cache memory so 64K context fits a 12–16 GB GPU. It shifts numerics slightly,
  so record it in `report.py --note` to keep the leaderboard row honest.
