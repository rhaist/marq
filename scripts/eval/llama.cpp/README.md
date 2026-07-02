# llama.cpp quickstarts for the eval sweep

Ready-to-run `llama-server` commands for the two reference Gemma configs. These
carry the same context-size / `temperature` / `top_p` / `top_k` / `min_p` /
`repeat_penalty` settings the profiles expect, passed as `llama-server` flags.
The harness then talks to
`http://localhost:8080/v1`.

## Uncensored "Balanced" (recommended)

HauhauCS Gemma-4-12B QAT Uncensored "Balanced" (Q4_K_M), its card's preset:

```bash
llama-server -hf HauhauCS/Gemma4-12B-QAT-Uncensored-HauhauCS-Balanced:Q4_K_M \
    --host 0.0.0.0 --port 8080 --ctx-size 65536 --jinja \
    --temp 0.6 --top-p 0.9 --top-k 64 --min-p 0.05 --repeat-penalty 1.1
```

## Stock Google Gemma-4-12B-IT (QAT Q4_0)

Google's official sampling:

```bash
llama-server -hf google/gemma-4-12B-it-qat-q4_0-gguf \
    --host 0.0.0.0 --port 8080 --ctx-size 65536 --jinja \
    --temp 1.0 --top-p 0.95 --top-k 64
```

## Notes

- `--jinja` enables the model's chat template so tool calls parse.
- `--ctx-size` must be large — tool outputs are big; 65536 matches the profiles.
- Verify the exact `-hf` quant tag on the model card (the `:Q4_K_M` suffix, etc.).
- Add `-ngl 99` to offload all layers to the GPU.
